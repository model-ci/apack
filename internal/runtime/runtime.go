package runtime

import (
	"context"

	"github.com/model-ci/apack/pkg/progress"
)

/*
	type runtime struct {
		distribution.Distribution
		//infer.Infer
	}
*/
type Runtime interface {
	Run(ctx context.Context, ref, path string, plog progress.Logger) error
	Kill(ctx context.Context, id string) error
	Ps(ctx context.Context) (*States, error)
}

/*
func New(d distribution.Distribution, workspace string) (Runtime, error) {
	client, err := infer.New(&infercfg.Config{
		AutoRestart: true,
		MaxRetries:  3,
		BinaryPath:  filepath.Join(workspace, "runtime"),
	})

	if err != nil {
		return nil, err
	}

	rt := &runtime{
		Distribution: d,
		Infer:        client,
	}

	return rt, nil
	return &runtime{}, nil
}

func (r *runtime) Run(ctx context.Context, ref, path string, plog progress.Logger) error {
	modelPath, err := findModelFile(path)
	if err != nil {
		return err
	}

	_ = modelPath

	pp := &PortPair{}
	if err := pp.GenPortPair(); err != nil {
		return err
	}


	runner, desc, err := r.Infer.Create(sc, ac, ref)
	if err != nil {
		return err
	}

	defer func(cause error) {
		if cause != nil {
			runner.Stop()
		}
	}(err)

	if err := runner.Start(ctx); err != nil {
		return err
	}

	_, _, err = r.Distribution.Manifest(ctx, ref)
	if err != nil {
		return err
	}

	st := &State{
		ID:         desc.Digest.Encoded()[:12],
		ModelImage: ref,
		CreateAt:   time.Now(),
		Status:     "",
		Endpoints: []string{
			fmt.Sprintf(":%d", sc.Port),
			fmt.Sprintf(":%d", ac.Port),
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
	runner, err := r.Infer.Get(id)
	if err != nil {
		return fmt.Errorf(
			"runtime %s not found")
	}

	if err := runner.Stop(); err != nil {
		return err
	}

	statuses, err := r.Distribution.Statuses(ctx)
	if err != nil {
		return err
	}

	found := false

	var stateBytes []byte
	var ref string
	for _, status := range statuses {
		if status.Digest.Encoded()[:12] == id {
			stateBytes = status.Data
			ref = status.Annotations[layerdb.OCIAnnotationRefName]
			found = true
		}
	}

	if !found && stateBytes == nil {
		return fmt.Errorf("runtime %s not found", id)
	}

	st := &State{}
	err = st.UnmarshalJSON(stateBytes)
	if err != nil {
		return err
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

		run, err := r.Get(st.ID)
		if err != nil {
			return nil, err
		}

		if run.IsRunning() {
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
*/
