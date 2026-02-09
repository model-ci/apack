package progress

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/task"
	"github.com/model-ci/apack/pkg/httputil"
)

type Client struct {
	serverURL string
	host      string
	scheme    string
	completed map[string]bool
}

func NewClient(serverURL string) *Client {
	client := &Client{
		serverURL: serverURL,
		host:      serverURL,
		completed: make(map[string]bool),
	}

	client.parseHost()
	return client
}

func NewClientFromHttpClient(httpClient *httputil.HttpClient) *Client {
	return &Client{
		serverURL: httpClient.GetBaseURL(),
		host:      httpClient.GetHost(),
		completed: make(map[string]bool),
	}
}

func (c *Client) parseHost() {
	switch {
	case strings.HasPrefix(c.host, "unix://"):
		c.scheme = "ws"
		c.serverURL = "ws://unix"
	case strings.HasPrefix(c.host, "tcp://"):
		host := strings.TrimPrefix(c.host, "tcp://")
		c.scheme = "ws"
		c.serverURL = "ws://" + host
	case strings.HasPrefix(c.host, "http://"):
		c.scheme = "ws"
		c.serverURL = strings.Replace(c.host, "http://", "ws://", 1)
	case strings.HasPrefix(c.host, "https://"):
		c.scheme = "wss"
		c.serverURL = strings.Replace(c.host, "https://", "wss://", 1)
	default:
		if strings.Contains(c.host, ":") {
			c.scheme = "ws"
			c.serverURL = "ws://" + c.host
		} else {
			c.scheme = "ws"
			c.serverURL = "ws://unix"
			c.host = "unix://" + c.host
		}
	}
}

func (c *Client) WatchProgress(ctx context.Context, taskID, operation string) error {
	return c.watcher(ctx, taskID, operation)
}

func (c *Client) Watcher(ctx context.Context, taskID, operation string) (<-chan *task.Event, error) {
	return c.watcherStream(ctx, taskID, operation)
}

func (c *Client) watcherStream(ctx context.Context, taskID, operation string) (<-chan *task.Event, error) {
	u := fmt.Sprintf("%s/base/v1/tasks/%s/stream", c.serverURL, taskID)

	dialCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	dialer := c.createDialer()
	conn, resp, err := websocket.Dial(dialCtx, u, &websocket.DialOptions{
		CompressionMode: websocket.CompressionContextTakeover,
		HTTPClient: &http.Client{
			Transport: &http.Transport{DialContext: dialer},
		},
	})

	if err != nil {
		if resp != nil {
			log.Logger.Errorf("WebSocket dial failed with status: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	conn.SetReadLimit(4 * 1024 * 1024)
	ch := make(chan *task.Event, 100)

	go func() {
		defer conn.CloseNow()
		defer close(ch)

		for {
			select {
			case <-ctx.Done():
				log.Logger.Infof("Context cancelled for %s", operation)
				return
			default:
			}

			_, data, err := conn.Read(ctx)
			if err != nil {
				if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
					log.Logger.Infof("WebSocket connection closed normally")
					return
				}

				log.Logger.Errorf("WebSocket read error: %v", err)

				select {
				case ch <- &task.Event{
					Type:    task.EventError,
					Message: fmt.Sprintf("read error: %v", err),
				}:
				case <-ctx.Done():
				}
				return
			}

			if len(data) == 0 {
				log.Logger.Warn("Received empty message")
				continue
			}

			var event task.Event
			if err := event.UnmarshalJSON(data); err != nil {
				log.Logger.Warnf("Failed to unmarshal event data: %s, error: %v", string(data), err)
				continue
			}

			select {
			case ch <- &event:
			case <-ctx.Done():
				return
			}

			switch event.Type {
			case task.EventError, task.EventComplete, task.EventCancel:
				return
			}
		}
	}()

	return ch, nil
}

func (c *Client) watcher(ctx context.Context, taskID, operation string) error {
	u := fmt.Sprintf("%s/base/v1/tasks/%s/stream", c.serverURL, taskID)
	dialer := c.createDialer()

	conn, resp, err := websocket.Dial(ctx, u, &websocket.DialOptions{
		CompressionMode: websocket.CompressionContextTakeover,
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				DialContext: dialer,
			},
		},
	})

	if err != nil {
		if resp != nil {
			log.Logger.Errorf("WebSocket dial failed with status: %d", resp.StatusCode)
		}
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	defer conn.CloseNow()

	for {
		select {
		case <-ctx.Done():
			log.Logger.Infof("Context cancelled for %s", operation)
			return ctx.Err()
		default:
		}

		_, data, err := conn.Read(ctx)

		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				log.Logger.Infof("WebSocket connection closed normally")
				return nil
			}
			log.Logger.Errorf("Failed to read from WebSocket: %v", err)
			return fmt.Errorf("failed to read progress event: %w", err)
		}

		if len(data) == 0 {
			log.Logger.Warn("Received empty message")
			continue
		}

		event := task.Event{}
		if err := event.UnmarshalJSON(data); err != nil {
			log.Logger.Warnf("Failed to unmarshal event data: %s, error: %v", string(data), err)
			continue
		}

		switch event.Type {
		case task.EventProgress:
			c.updateProgress(event)

		case task.EventLog:
			c.handleLogEvent(event)

		case task.EventError:
			return fmt.Errorf(event.Message)

		case task.EventComplete:
			fmt.Printf("%s\n", strings.Trim(event.Message, "{}"))
			return nil

		case task.EventCancel:
			fmt.Println()
			return fmt.Errorf("%s was canceled", operation)
		}
	}
}

func (c *Client) createDialer() func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		switch {
		case strings.HasPrefix(c.host, "unix://"):
			socketPath := strings.TrimPrefix(c.host, "unix://")
			return net.DialTimeout("unix", socketPath, 30*time.Second)

		case strings.HasPrefix(c.host, "tcp://"):
			host := strings.TrimPrefix(c.host, "tcp://")
			return net.DialTimeout("tcp", host, 30*time.Second)

		default:
			dialer := &net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}
}

func (c *Client) WatchProgressWithRetry(ctx context.Context, taskID, operation string) error {
	const maxRetries = 3
	const retryDelay = time.Second * 2

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := c.watcher(ctx, taskID, operation)

		if err == nil {
			return nil
		}

		if websocket.CloseStatus(err) == websocket.StatusNormalClosure || ctx.Err() != nil {
			return err
		}

		if attempt < maxRetries {
			log.Logger.Warnf("Connection lost (attempt %d/%d), retrying in %v: %v",
				attempt, maxRetries, retryDelay, err)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay * time.Duration(attempt)):
				continue
			}
		}
	}

	return fmt.Errorf("failed after %d attempts", maxRetries)
}

func (c *Client) updateProgress(event task.Event) {
	barKey := event.Identifier
	if barKey == "" || event.Total == 0 {
		return
	}

	if c.completed == nil {
		c.completed = make(map[string]bool)
	}

	if c.completed[barKey] {
		return
	}

	isComplete := event.Progress >= event.Total ||
		(event.Total > 0 && float64(event.Progress)/float64(event.Total) >= 0.999)

	if isComplete {
		c.completed[barKey] = true

		if event.Total > 1024*1024 {
			fmt.Printf("\r%s\r", strings.Repeat(" ", 80))
		}

		sizeStr := FormatSize(event.Total)
		fmt.Printf("%s: %-20s ✓ COMPLETE %s\n", event.Action, barKey, sizeStr)
	} else {
		if event.Total > 1024*1024 {
			percentage := float64(event.Progress) / float64(event.Total) * 100
			sizeStr := FormatSize(event.Total)
			fmt.Printf("\r%s: %-20s [%.1f%%] %s", event.Action, barKey, percentage, sizeStr)
		}
	}
}

func (c *Client) handleLogEvent(event task.Event) {
	switch event.Level {
	case task.LogLevelDebug:
		log.Logger.Debug(event.Message)
	case task.LogLevelInfo:
		log.Logger.Info(event.Message)
	case task.LogLevelWarn:
		log.Logger.Warn(event.Message)
	case task.LogLevelError:
		log.Logger.Error(event.Message)
	}
}
