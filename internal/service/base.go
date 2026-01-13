package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/model-ci/apack/internal/config"
	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/repo"
	"github.com/model-ci/apack/internal/runtime"
	"github.com/model-ci/apack/internal/spec"
	"github.com/model-ci/apack/internal/task"
	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/pkg/distribution"

	"github.com/model-ci/apack/pkg/layerdb"
	"github.com/model-ci/apack/pkg/progress"
	"github.com/model-ci/apack/pkg/tools"
	"go.uber.org/zap"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
)

type BaseService struct {
	path         string
	manager      *task.Manager
	distribution distribution.Distribution
	runtime      runtime.Runtime
	logger       *zap.SugaredLogger
	config       *Config
	tools        *tools.Tools
}

func NewBaseService(path string, m *task.Manager, config *Config) (*BaseService, error) {
	db, err := layerdb.Open(path, nil)
	if err != nil {
		return nil, err
	}

	d, err := repo.New(path, db)
	if err != nil {
		return nil, err
	}
	rt, err := runtime.New(d, path)
	if err != nil {
		return nil, err
	}

	t, err := tools.NewTools(db, d, config.Concurrency)
	if err != nil {
		return nil, err
	}

	return &BaseService{
		path:         path,
		distribution: d,
		runtime:      rt,
		manager:      m,
		config:       config,
		tools:        t,
	}, nil
}

func (b *BaseService) Push(ctx context.Context, req *types.Request) (*types.Response, error) {
	task := b.manager.CreateTask("push", map[string]interface{}{
		"reference": req.Reference,
	})

	go b.executePush(task.Context(), task.ID, req)

	return &types.Response{
		TaskID: task.ID,
	}, nil
}

func (b *BaseService) Gen(ctx context.Context, req *types.Request) (*types.Response, error) {
	artifactPath := filepath.Join(req.Artifact.Package.Workspace, spec.DefaultArtifactName)
	if _, err := os.Stat(artifactPath); err == nil {
		if !req.Overwrite {
			return nil, fmt.Errorf("apackfile already exists at %s.", artifactPath)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("Error checking for existing artifact: %s", err)
	}

	dirContents, err := spec.DirectoryFromFS(req.Artifact.Package.Workspace)
	if err != nil {
		return nil, fmt.Errorf("Error processing directory: %s", err)
	}
	artifact, err := spec.Gen(dirContents, &req.Artifact)
	if err != nil {
		return nil, fmt.Errorf("Error generating artifact: %s", err)
	}
	bytes, err := artifact.MarshalToYAML()
	if err != nil {
		return nil, fmt.Errorf("Error formatting artifact: %s", err)
	}
	return &types.Response{
		Message: bytes,
	}, nil
}

func (b *BaseService) Pull(ctx context.Context, req *types.Request) (*types.Response, error) {
	task := b.manager.CreateTask("pull", map[string]interface{}{
		"reference": req.Reference,
	})

	go b.executePull(task.Context(), task.ID, req)

	return &types.Response{
		TaskID: task.ID,
	}, nil
}

func (b *BaseService) Build(ctx context.Context, req *types.Request) (*types.Response, error) {
	task := b.manager.CreateTask("build", map[string]interface{}{
		"reference": req.Reference,
	})

	log.Logger.Debugf("task %s: building %s", task.ID, req.Reference)

	go b.executeBuild(task.Context(), task.ID, req)

	return &types.Response{
		TaskID: task.ID,
	}, nil
}

func (b *BaseService) Tag(ctx context.Context, req *types.Request) (*types.Response, error) {
	err := b.distribution.Tag(ctx, req.Reference, req.Params.TargetRef)
	if err != nil {
		return nil, err
	}
	return &types.Response{}, nil
}

func (b *BaseService) Export(ctx context.Context, req *types.Request) (*types.Response, error) {
	task := b.manager.CreateTask("export", map[string]interface{}{
		"reference": req.Reference,
	})

	go b.executeExport(task.Context(), task.ID, req)

	return &types.Response{
		TaskID: task.ID,
	}, nil
}

func (b *BaseService) Import(ctx context.Context, req *types.Request) (*types.Response, error) {
	task := b.manager.CreateTask("import", map[string]interface{}{
		"reference": req.Reference,
	})

	go b.executeImport(task.Context(), task.ID, req)

	return &types.Response{
		TaskID: task.ID,
	}, nil
}

func (b *BaseService) List(ctx context.Context, _ *types.Request) (*types.Response, error) {
	items, err := b.distribution.Artifacts(ctx)
	if err != nil {
		return nil, err
	}

	artifacts := types.Artifacts{Items: items, Count: len(items)}
	artifactsBytes, err := artifacts.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return &types.Response{
		Message: artifactsBytes,
	}, nil
}

func (b *BaseService) Info(ctx context.Context, req *types.Request) (*types.Response, error) {
	artifact, err := b.distribution.Artifact(ctx, req.ReferenceStr)
	if err != nil {
		return nil, err
	}

	artifactBytes, err := artifact.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return &types.Response{
		Message: artifactBytes,
	}, nil
}

func (b *BaseService) Inspect(ctx context.Context, req *types.Request) (*types.Response, error) {
	mf, _, err := b.distribution.Manifest(ctx, req.ReferenceStr)
	if err != nil {
		return nil, err
	}

	config, err := b.distribution.Config(ctx, mf.Config)
	if err != nil {
		return nil, err
	}

	artifact, err := b.distribution.Artifact(ctx, req.ReferenceStr)
	if err != nil {
		return nil, err
	}

	inspect := &types.Inspect{
		Manifest: layerdb.Manifest{Manifest: mf},
		Config:   config,
		Artifact: artifact,
	}

	inspectBytes, err := inspect.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return &types.Response{
		Message: inspectBytes,
	}, nil
}

func (b *BaseService) Remove(ctx context.Context, req *types.Request) (*types.Response, error) {
	err := b.distribution.Remove(ctx, req.ReferenceStr)
	if err != nil {
		return nil, err
	}

	return &types.Response{}, nil
}

func (b *BaseService) Run(ctx context.Context, req *types.Request) (*types.Response, error) {
	task := b.manager.CreateTask("run", map[string]interface{}{
		"reference": req.ReferenceStr,
	})

	go b.executeRun(task.Context(), task.ID, req)

	return &types.Response{
		TaskID: task.ID,
	}, nil
}

func (b *BaseService) executeRun(ctx context.Context, taskID string, req *types.Request) {
	defer func() {
		if r := recover(); r != nil {
			b.manager.CompleteTask(taskID, fmt.Errorf("model run panic: %v", r))
		}
	}()

	progressLogger := progress.NewLogger(taskID, b.manager, b.logger)
	progressLogger.Infoln("Starting run model operation")

	path, err := b.distribution.Snapshot(ctx, req.ReferenceStr)
	if err != nil {
		b.manager.CompleteTask(taskID, err)
		return
	}
	err = b.doRun(ctx, req.ReferenceStr, path, progressLogger)
	b.manager.CompleteTask(taskID, err)
}

func (b *BaseService) doRun(ctx context.Context, ref, path string, plog *progress.Logger) error {
	return b.runtime.Run(ctx, ref, path, *plog)
}

func (b *BaseService) Ps(ctx context.Context, req *types.Request) (*types.Response, error) {
	states, err := b.runtime.Ps(ctx)
	if err != nil {
		return nil, err
	}
	statesBytes, err := states.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return &types.Response{
		Message: statesBytes,
	}, nil
}

func (b *BaseService) Kill(ctx context.Context, req *types.Request) (*types.Response, error) {
	err := b.runtime.Kill(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return &types.Response{}, nil
}

func (b *BaseService) executePush(ctx context.Context, taskID string, req *types.Request) {
	defer func() {
		if r := recover(); r != nil {
			b.manager.CompleteTask(taskID, fmt.Errorf("push panic: %v", r))
		}
	}()

	progressLogger := progress.NewLogger(taskID, b.manager, b.logger)
	log.Logger.Debug("Starting push operation")

	err := b.doPush(ctx, progressLogger, req)
	b.manager.CompleteTask(taskID, err)
}

func (b *BaseService) executePull(ctx context.Context, taskID string, req *types.Request) {
	defer func() {
		if r := recover(); r != nil {
			b.manager.CompleteTask(taskID, fmt.Errorf("pull panic: %v", r))
		}
	}()

	progressLogger := progress.NewLogger(taskID, b.manager, b.logger)
	log.Logger.Debug("Starting pull operation")

	err := b.doPull(ctx, progressLogger, req)
	b.manager.CompleteTask(taskID, err)
}

func (b *BaseService) executeBuild(ctx context.Context, taskID string, req *types.Request) {
	defer func() {
		if r := recover(); r != nil {
			b.manager.CompleteTask(taskID, fmt.Errorf("build panic: %v", r))
		}
	}()

	progressLogger := progress.NewLogger(taskID, b.manager, b.logger)
	log.Logger.Debug("Starting build operation")

	err := b.doBuild(ctx, progressLogger, req)
	b.manager.CompleteTask(taskID, err)
}

func (b *BaseService) executeExport(ctx context.Context, taskID string, req *types.Request) {
	defer func() {
		if r := recover(); r != nil {
			b.manager.CompleteTask(taskID, fmt.Errorf("export panic: %v", r))
		}
	}()

	progressLogger := progress.NewLogger(taskID, b.manager, b.logger)
	log.Logger.Debug("Starting export operation")

	log.Logger.Debugf("req: %+v", req)

	err := b.doExport(ctx, progressLogger, req)
	b.manager.CompleteTask(taskID, err)
}

func (b *BaseService) executeImport(ctx context.Context, taskID string, req *types.Request) {
	defer func() {
		if r := recover(); r != nil {
			b.manager.CompleteTask(taskID, fmt.Errorf("import panic: %v", r))
		}
	}()

	progressLogger := progress.NewLogger(taskID, b.manager, b.logger)
	log.Logger.Debug("Starting import operation")

	log.Logger.Debugf("req: %+v", req)

	err := b.doImport(ctx, progressLogger, req)
	b.manager.CompleteTask(taskID, err)
}

func (b *BaseService) doPush(ctx context.Context, plog *progress.Logger, req *types.Request) error {
	var reg *remote.Registry
	var err error

	if req.ConfigJSON != nil {
		reg, err = repo.NewRegistryRemote(req.Reference.Registry, &distribution.Options{ConfigJSON: req.ConfigJSON})
		if err != nil {
			log.Logger.Errorf("failed to create remote registry: %s", err.Error())
			return err
		}
	} else {
		configPath := config.JsonPath("")
		reg, err = repo.NewRegistry(req.Reference.Registry, &distribution.Options{CredentialsPath: configPath, PlainHTTP: true})
		if err != nil {
			log.Logger.Errorf("failed to create registry: %s", err.Error())
			return err
		}
	}

	remoteRepo, err := reg.Repository(ctx, req.Reference.Repository)
	if err != nil {
		return fmt.Errorf("failed to create repository reference: %w", err)
	}

	tag := req.Reference.Reference
	if tag == "" {
		tag = "latest"
	}

	log.Logger.Infoln(fmt.Sprintf("Resolving %s:%s", req.Reference.Repository, tag))

	desc, err := b.distribution.Push(ctx, remoteRepo, registry.Reference{
		Registry:   req.Reference.Registry,
		Repository: req.Reference.Repository,
		Reference:  tag,
	}, plog, &distribution.Options{
		Concurrency: b.config.Concurrency,
	})

	if err != nil {
		return fmt.Errorf("failed to push %s:%s: %w", req.Reference.Repository, tag, err)
	}

	log.Logger.Infoln(fmt.Sprintf("Successfully pushed %s:%s", req.Reference.Repository, tag))
	log.Logger.Infoln(fmt.Sprintf("Manifest digest: %s", desc.Digest.Encoded()))
	plog.InfolnWithAction(types.Description{Action: "Pushed", Digest: "digest: " + desc.Digest.String(), Size: fmt.Sprintf("size: %d", desc.Size)}, task.EventComplete)

	return nil
}

func (b *BaseService) doPull(ctx context.Context, plog *progress.Logger, req *types.Request) error {
	var reg *remote.Registry
	var err error

	if req.ConfigJSON != nil {
		reg, err = repo.NewRegistryRemote(req.Reference.Registry, &distribution.Options{ConfigJSON: req.ConfigJSON})
		if err != nil {
			log.Logger.Errorf("failed to create remote registry: %s", err.Error())
			return err
		}
	} else {
		configPath := config.JsonPath("")
		reg, err = repo.NewRegistry(req.Reference.Registry, &distribution.Options{CredentialsPath: configPath, PlainHTTP: true})
		if err != nil {
			log.Logger.Errorf("failed to create registry: %s", err.Error())
			return err
		}
	}

	remoteRepo, err := reg.Repository(ctx, req.Reference.Repository)
	if err != nil {
		return fmt.Errorf("failed to create repository reference: %w", err)
	}

	tag := req.Reference.Reference
	if tag == "" {
		tag = "latest"
	}

	log.Logger.Infoln(fmt.Sprintf("Resolving %s:%s", req.Reference.Repository, tag))

	desc, err := b.distribution.Pull(ctx, remoteRepo, registry.Reference{
		Registry:   req.Reference.Registry,
		Repository: req.Reference.Repository,
		Reference:  tag,
	}, plog, &distribution.Options{
		Concurrency: b.config.Concurrency,
	})

	if err != nil {
		return fmt.Errorf("failed to pull %s:%s: %w", req.Reference.Repository, tag, err)
	}

	log.Logger.Infoln(fmt.Sprintf("Successfully pulled %s:%s", req.Reference.Repository, tag))
	log.Logger.Infoln(fmt.Sprintf("Manifest digest: %s", desc.Digest.Encoded()))
	plog.InfolnWithAction(types.Description{Action: "Pulled", Digest: "digest: " + desc.Digest.String(), Size: fmt.Sprintf("size: %d", desc.Size)}, task.EventComplete)

	return nil
}

func (b *BaseService) doBuild(ctx context.Context, plog *progress.Logger, req *types.Request) error {
	makefile := repo.NewMakefile(req.Artifact, req.Reference.String(), b.config.GetCompress(), true)
	desc, err := b.distribution.Bundle(ctx, makefile, plog, &distribution.Options{Concurrency: b.config.Concurrency})
	if err != nil {
		return fmt.Errorf("failed to build: %w", err)
	}

	tag := req.Reference.Reference
	if tag == "" {
		tag = "latest"
	}

	log.Logger.Infoln(fmt.Sprintf("Successfully build %s:%s", req.Reference.Repository, req.Reference.Reference))
	log.Logger.Infoln(fmt.Sprintf("Manifest digest: %s", desc.Digest.Encoded()))
	plog.InfolnWithAction(types.Description{Action: "Builded", Digest: "digest: " + desc.Digest.String(), Size: fmt.Sprintf("size: %d", desc.Size)}, task.EventComplete)

	return nil
}

func (b *BaseService) doExport(ctx context.Context, plog *progress.Logger, req *types.Request) error {
	err := b.distribution.Extract(ctx, req.Output, req.ReferenceStr, plog, &distribution.Options{Concurrency: b.config.Concurrency})
	if err != nil {
		return fmt.Errorf("failed to export: %w", err)
	}
	log.Logger.Debug(fmt.Sprintf("Successfully exported %s output: %s", req.ReferenceStr, req.Output))
	return nil
}

func (b *BaseService) doImport(ctx context.Context, plog *progress.Logger, req *types.Request) error {
	tool, err := b.tools.Get(req.User, req.Password, req.Tool)
	if err != nil {
		return err
	}

	desc, err := tool.Fetch(ctx, req.ReferenceStr, b.distribution.Snappath(ctx, req.ReferenceStr), plog)
	if err != nil {
		return err
	}

	log.Logger.Infoln(fmt.Sprintf("Successfully fetch %s:%s, tools: %s", req.Reference.Repository, req.Reference.Reference, req.Tool))
	log.Logger.Infoln(fmt.Sprintf("Manifest digest: %s", desc.Digest.Encoded()))
	plog.InfolnWithAction(types.Description{Action: "Fetched", Digest: "digest: " + desc.Digest.String(), Size: fmt.Sprintf("size: %d", desc.Size)}, task.EventComplete)
	return nil
}
