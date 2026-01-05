package utils

import (
	"fmt"
	"time"

	"github.com/model-ci/apack/pkg/client"
	"github.com/model-ci/apack/pkg/progress"
)

type Client struct {
	*client.HttpClient
	*progress.Client
}

func NewDefaultClient(host string) (*Client, error) {
	return NewClient(client.HttpClientOptions{
		Host:    host,
		Timeout: 30 * time.Second,
	})
}

func NewClient(opts client.HttpClientOptions) (*Client, error) {
	c := &Client{}
	httpClient, err := client.NewHttpClient(opts)
	if err != nil {
		return nil, fmt.Errorf("http client create error: %s", err)
	}

	c.HttpClient = httpClient

	c.Client = progress.NewClientFromHttpClient(httpClient)
	return c, nil
}

func (c *Client) Close() error {
	if c.HttpClient != nil {
		return c.HttpClient.Close()
	}
	return nil
}
