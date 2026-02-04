package runtime

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/runtime/infer"
	"github.com/model-ci/apack/internal/spec"
	"github.com/model-ci/apack/pkg/distribution"
	"github.com/model-ci/apack/pkg/layerdb"
)

type Runtime interface {
	Run(ctx context.Context, ref, path string, models []spec.Model) error
	Kill(ctx context.Context, id string) error
	Ps(ctx context.Context) (*States, error)
}

type runtime struct {
	distribution.Distribution
	*infer.Infer
}

func New(d distribution.Distribution, workspace string) (Runtime, error) {
	cfg := &infer.Config{
		AutoRestart: true,
		MaxRetries:  3,
		BasePath:    filepath.Join(workspace, "runtime"),
	}

	cfg.LogPath = filepath.Join(cfg.BasePath, "logs")
	cfg.PidPath = filepath.Join(cfg.BasePath, "pids")

	client, err := infer.New(cfg)
	if err != nil {
		return nil, err
	}

	rt := &runtime{
		Distribution: d,
		Infer:        client,
	}

	return rt, nil
}

func (r *runtime) Run(ctx context.Context, ref, path string, models []spec.Model) error {
	if models == nil || len(models) > 1 {
		log.Logger.Debug("Only one model can be run at a time")
		return fmt.Errorf("only one model can be run at a time")
	}

	modelid := models[0].ID
	modelpath := filepath.Join(path, modelid)

	log.Logger.Debugf("Running model %s", modelid)

	pp := &PortPair{}
	if err := pp.GenPortPair(); err != nil {
		return err
	}

	proc, desc, err := r.Infer.Spawning(&infer.Params{
		ID:        modelid,
		ModelPath: modelpath,
		Port:      pp.Port1,
		Reference: ref,
	})
	if err != nil {
		return err
	}

	defer func(cause error) {
		if cause != nil {
			r.Infer.Stop(shortid(modelid))
		}
	}(err)

	if err := proc.Exec(ctx); err != nil {
		return err
	}

	_, _, err = r.Distribution.Manifest(ctx, ref)
	if err != nil {
		return err
	}

	st := &State{
		ID:         shortid(modelid),
		ModelImage: ref,
		CreateAt:   time.Now(),
		Endpoints: []string{
			fmt.Sprintf(":%d", pp.Port1),
			fmt.Sprintf(":%s", "--"),
		},
		Descriptor: desc,
	}

	st.UpdateStatus(StatusCreated, st.CreateAt)

	stateBytes, err := st.MarshalJSON()
	if err != nil {
		return err
	}

	stateDesc := layerdb.ConfigDesc(stateBytes, OCIAnnotationRuntimeState)
	stateDesc.Data = stateBytes

	return r.Distribution.State(ctx, ref, stateDesc)
}

func (r *runtime) Kill(ctx context.Context, id string) error {
	if err := r.Infer.Stop(id); err != nil {
		return err
	}

	statuses, err := r.Distribution.Statuses(ctx)
	if err != nil {
		return err
	}

	found := false

	var stateBytes []byte
	var ref string
	st := &State{}
	for _, status := range statuses {
		err = st.UnmarshalJSON(status.Data)
		if err != nil {
			return err
		}

		if st.ID == id {
			ref = status.Annotations[layerdb.OCIAnnotationRefName]
			found = true
		}
	}

	if !found {
		return fmt.Errorf("runtime %s not found", id)
	}

	st.UpdateStatus(StatusExited, st.CreateAt)
	stateBytes, err = st.MarshalJSON()
	if err != nil {
		return err
	}

	stateDesc := layerdb.ConfigDesc(stateBytes, OCIAnnotationRuntimeState)
	stateDesc.Data = stateBytes
	stateDesc.Annotations = map[string]string{
		layerdb.OCIAnnotationRefName: ref,
	}

	return r.Distribution.State(ctx, ref, stateDesc)
}

func (r *runtime) Ps(ctx context.Context) (*States, error) {
	states := &States{Count: 0}
	statuses, err := r.Distribution.Statuses(ctx)
	if err != nil {
		return nil, err
	}

	for _, status := range statuses {
		st := &State{}
		err = st.UnmarshalJSON(status.Data)
		if err != nil {
			return nil, err
		}

		proc, exist := r.Status(st.ID)
		if !exist {
			return nil, fmt.Errorf("runtime %s not found", st.ID)
		}

		if proc.IsRunning() {
			st.UpdateStatus(StatusUp, st.CreateAt)
		} else {
			st.UpdateStatus(StatusExited, st.CreateAt)
		}

		stateBytes, err := st.MarshalJSON()
		if err != nil {
			return nil, err
		}

		stateDesc := layerdb.ConfigDesc(stateBytes, OCIAnnotationRuntimeState)
		stateDesc.Data = stateBytes
		err = r.Distribution.State(ctx, st.ModelImage, stateDesc)
		if err != nil {
			return nil, err
		}

		states.Items = append(states.Items, *st)
		states.Count++
	}

	return states, nil
}
