package repo

import (
	"fmt"
	"net/http"
	"time"

	"github.com/model-ci/apack/internal/log"
	"oras.land/oras-go/v2/registry/remote"
)

type LoggerClient struct {
	remote.Client
	name string
}

func (c *LoggerClient) Do(req *http.Request) (*http.Response, error) {
	start := time.Now()

	clientPrefix := ""
	if c.name != "" {
		clientPrefix = fmt.Sprintf("[%s] ", c.name)
	}

	contentLength := req.ContentLength
	if contentLength > 0 {
		log.Logger.Infof("%s%s %s (Content-Length: %d bytes)",
			clientPrefix, req.Method, req.URL.Redacted(), contentLength)
	} else {
		log.Logger.Infof("%s%s %s",
			clientPrefix, req.Method, req.URL.Redacted())
	}

	resp, err := c.Client.Do(req)
	duration := time.Since(start)

	if err != nil {
		if ctx := req.Context(); ctx != nil && ctx.Err() != nil {
			log.Logger.Infof("%s%s %s -> CANCELLED (context: %v) -- duration %v",
				clientPrefix, req.Method, req.URL.Redacted(), ctx.Err(), duration)
		} else {
			log.Logger.Infof("%s%s %s -> ERROR (%v) -- duration %v",
				clientPrefix, req.Method, req.URL.Redacted(), err, duration)
		}
	} else {
		responseInfo := fmt.Sprintf("-> %d", resp.StatusCode)
		if resp.ContentLength > 0 {
			responseInfo += fmt.Sprintf(" (%d bytes)", resp.ContentLength)
		}
		log.Logger.Infof("%s%s %s %s -- duration %v",
			clientPrefix, req.Method, req.URL.Redacted(), responseInfo, duration)
	}

	return resp, err
}

func WrapClient(c remote.Client, debug bool) remote.Client {
	return WrapClientWithName(c, "", debug)
}

func WrapClientWithName(c remote.Client, name string, debug bool) remote.Client {
	if debug {
		return &LoggerClient{
			Client: c,
			name:   name,
		}
	}
	return c
}
