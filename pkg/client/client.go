package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/model-ci/apack/internal/task"
	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/pkg/httputil"
	"github.com/model-ci/apack/pkg/progress"
	"github.com/model-ci/apack/pkg/tools/huggingface"
	"github.com/model-ci/apack/pkg/tools/ollama"
	"oras.land/oras-go/v2/registry"
)

type Client interface {
	Import(ctx context.Context, reference string, tag string, tool string, endpoint string) (*types.Response, error)
	Push(ctx context.Context, reference string) (*types.Response, error)
	Tag(ctx context.Context, srcref string, dstref string) error
	Watcher(ctx context.Context, taskID string, action string) (<-chan *task.Event, error)
}

type BaseClient struct {
	host string
	*progress.Client
	*httputil.HttpClient
}

func NewDefaultCLI(host string) (*BaseClient, error) {
	return NewClient(httputil.HttpClientOptions{
		Host:    host,
		Timeout: 30 * time.Second,
	})
}

func NewDefaultSDK(host string) (Client, error) {
	return NewClient(httputil.HttpClientOptions{
		Host:    host,
		Timeout: 30 * time.Second,
	})
}

func NewClient(opts httputil.HttpClientOptions) (*BaseClient, error) {
	c := &BaseClient{}
	httpClient, err := httputil.NewHttpClient(opts)
	if err != nil {
		return nil, fmt.Errorf("http client create error: %s", err)
	}

	c.HttpClient = httpClient

	c.Client = progress.NewClientFromHttpClient(httpClient)
	return c, nil
}

func (c *BaseClient) Close() error {
	if c.HttpClient != nil {
		return c.HttpClient.Close()
	}
	return nil
}

func (c *BaseClient) Import(ctx context.Context, reference string, tag string, tool string, endpoint string) (*types.Response, error) {
	var repo string
	if reference == "" {
		return nil, fmt.Errorf("artifact reference is required")
	} else {
		repo = reference
	}

	var ref registry.Reference
	var err error
	switch tool {
	case ollama.Name:
		ref = ollama.MakeReference(reference)
		ref.Registry = endpoint
	case huggingface.Name:
		reference = fmt.Sprintf("%s/%s:%s", endpoint, strings.ToLower(reference), tag)
		ref, err = registry.ParseReference(reference)
		if err != nil {
			return nil, err
		}
	}

	if err := ref.Validate(); err != nil {
		return nil, err
	}

	req := types.Request{
		Args: types.Args{
			User:     "apack",
			Password: "ai",
			Tool:     tool,
			Endpoint: endpoint,
			Repo:     repo,
			Branch:   tag,
		},
		Reference:    ref,
		ReferenceStr: ref.String(),
	}

	resp, err := c.Post(ctx, "/base/v1/import", req)
	if err != nil {
		return nil, err
	}

	res := &types.Response{}
	if err := res.Decode(resp.Body); err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return res, nil
	default:
		return nil, fmt.Errorf("Failed to import artifacts: %s", res.Message)
	}
}

func (c *BaseClient) Push(ctx context.Context, reference string) (*types.Response, error) {
	ref, err := registry.ParseReference(reference)
	if err != nil {
		return nil, err
	}

	req := types.Request{
		Reference: ref,
	}

	resp, err := c.Post(ctx, "/base/v1/push", req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	res := &types.Response{}
	if err := res.Decode(resp.Body); err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return res, nil
	default:
		return nil, fmt.Errorf("Failed to push model image: %s", res.Message)
	}
}

func (c *BaseClient) Tag(ctx context.Context, srcref string, dstref string) error {
	srcRef, err := registry.ParseReference(srcref)
	if err != nil {
		return fmt.Errorf("invalid source artifact reference %q: %w", srcref, err)
	}

	dstRef, err := registry.ParseReference(dstref)
	if err != nil {
		return fmt.Errorf("invalid target artifact reference %q: %w", dstref, err)
	}

	req := types.Request{
		Args: types.Args{
			TargetRef: dstRef,
		},
		ReferenceStr: srcRef.String(),
		Reference:    srcRef,
	}

	resp, err := c.Post(ctx, "/base/v1/tag", req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	res := &types.Response{}
	if err := res.Decode(resp.Body); err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	default:
		return fmt.Errorf("Failed to tag model: %s", res.Message)
	}
}

// for event := range stream {
//     switch event.Type {
//     case task.EventLog:
//         fmt.Println(event.Message)
//     case task.EventError:
//         return fmt.Errorf("task failed or connection lost: %s", event.Message)
//     case task.EventComplete:
//         fmt.Println("Done")
//     }
// }

func (c *BaseClient) Watcher(ctx context.Context, taskID string, action string) (<-chan *task.Event, error) {
	return c.Watcher(ctx, taskID, action)
}
