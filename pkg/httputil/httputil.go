package httputil

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HttpClient struct {
	httpHttpClient *http.Client
	baseURL        string
	host           string
	scheme         string
	timeout        time.Duration
}

type HttpClientOptions struct {
	Host            string
	Timeout         time.Duration
	TLSConfig       *tls.Config
	SkipTLSVerify   bool
	MaxIdleConns    int
	IdleConnTimeout time.Duration
}

func NewHttpClient(opts HttpClientOptions) (*HttpClient, error) {
	if opts.Host == "" {
		opts.Host = "unix:///tmp/run/apack.sock"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.MaxIdleConns == 0 {
		opts.MaxIdleConns = 10
	}
	if opts.IdleConnTimeout == 0 {
		opts.IdleConnTimeout = 90 * time.Second
	}

	client := &HttpClient{
		host:    opts.Host,
		timeout: opts.Timeout,
	}

	if err := client.parseHost(); err != nil {
		return nil, fmt.Errorf("failed to parse host: %v", err)
	}

	client.httpHttpClient = client.createHTTPHttpClient(opts)
	return client, nil
}

func (c *HttpClient) parseHost() error {
	host := c.host

	switch {
	case strings.HasPrefix(host, "unix://"):
		c.scheme = "http"
		c.baseURL = "http://unix"
		return nil
	case strings.HasPrefix(host, "tcp://"):
		host = strings.TrimPrefix(host, "tcp://")
		c.scheme = "http"
		c.baseURL = "http://" + host
		return nil
	case strings.HasPrefix(host, "http://"):
		c.scheme = "http"
		c.baseURL = host
		return nil
	case strings.HasPrefix(host, "https://"):
		c.scheme = "https"
		c.baseURL = host
		return nil
	default:
		if strings.Contains(host, ":") {
			c.scheme = "http"
			c.baseURL = "http://" + host
		} else {
			c.scheme = "http"
			c.baseURL = "http://unix"
			c.host = "unix://" + host
		}
		return nil
	}
}

func (c *HttpClient) createHTTPHttpClient(opts HttpClientOptions) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        opts.MaxIdleConns,
		IdleConnTimeout:     opts.IdleConnTimeout,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableCompression:  false,
	}

	if strings.HasPrefix(c.host, "unix://") {
		socketPath := strings.TrimPrefix(c.host, "unix://")
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.DialTimeout("unix", socketPath, c.timeout)
		}
	} else {
		transport.DialContext = (&net.Dialer{
			Timeout:   c.timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext

		if c.scheme == "https" {
			if opts.TLSConfig != nil {
				transport.TLSClientConfig = opts.TLSConfig
			} else {
				transport.TLSClientConfig = &tls.Config{
					InsecureSkipVerify: opts.SkipTLSVerify,
				}
			}
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   c.timeout,
	}
}

func (c *HttpClient) Get(ctx context.Context, path string) (*http.Response, error) {
	return c.doRequest(ctx, "GET", path, nil)
}

func (c *HttpClient) Post(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	return c.doRequest(ctx, "POST", path, body)
}

func (c *HttpClient) Put(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	return c.doRequest(ctx, "PUT", path, body)
}

func (c *HttpClient) Delete(ctx context.Context, path string) (*http.Response, error) {
	return c.doRequest(ctx, "DELETE", path, nil)
}

func (c *HttpClient) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	fullURL, err := c.buildURL(path)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL: %v", err)
	}

	req, err := c.createRequest(ctx, method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := c.httpHttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}

	return resp, nil
}

func (c *HttpClient) buildURL(path string) (string, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	baseURL, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}

	pathURL, err := url.Parse(path)
	if err != nil {
		return "", err
	}

	return baseURL.ResolveReference(pathURL).String(), nil
}

func (c *HttpClient) createRequest(ctx context.Context, method, url string, body interface{}) (*http.Request, error) {
	var reqBody io.Reader

	if body != nil {
		switch v := body.(type) {
		case io.Reader:
			reqBody = v
		case []byte:
			reqBody = bytes.NewReader(v)
		case string:
			reqBody = strings.NewReader(v)
		default:
			jsonData, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal body: %v", err)
			}
			reqBody = bytes.NewReader(jsonData)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "apack-client/1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

func (c *HttpClient) Close() error {
	if c.httpHttpClient != nil {
		c.httpHttpClient.CloseIdleConnections()
	}
	return nil
}

func (c *HttpClient) GetBaseURL() string {
	return c.baseURL
}

func (c *HttpClient) GetHost() string {
	return c.host
}

func (c *HttpClient) IsRemote() bool {
	return c.isRemoteConnection()
}

func (c *HttpClient) IsLocal() bool {
	return !c.isRemoteConnection()
}

func (c *HttpClient) isRemoteConnection() bool {
	host := c.host

	switch {
	case strings.HasPrefix(host, "unix://"):
		return false

	case strings.HasPrefix(host, "tcp://"):
		address := strings.TrimPrefix(host, "tcp://")
		return c.isRemoteAddress(address)

	case strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://"):
		if u, err := url.Parse(host); err == nil {
			return c.isRemoteAddress(u.Host)
		}
		return true

	default:
		if strings.Contains(host, ":") {
			return c.isRemoteAddress(host)
		}
		return false
	}
}

func (c *HttpClient) isRemoteAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}

	return !c.isLocalHost(host)
}

func (c *HttpClient) isLocalHost(host string) bool {
	localHosts := []string{
		"localhost",
		"127.0.0.1",
		"::1",
		"0.0.0.0",
	}

	for _, localHost := range localHosts {
		if strings.EqualFold(host, localHost) {
			return true
		}
	}

	return c.isLocalIP(host)
}

func (c *HttpClient) isLocalIP(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	if ip.IsLoopback() {
		return true
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return false
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var localIP net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				localIP = v.IP
			case *net.IPAddr:
				localIP = v.IP
			}

			if localIP != nil && localIP.Equal(ip) {
				return true
			}
		}
	}

	return false
}

func (c *HttpClient) GetConnectionType() string {
	if c.IsLocal() {
		return "local"
	}
	return "remote"
}

func (c *HttpClient) GetConnectionInfo() map[string]interface{} {
	return map[string]interface{}{
		"host":     c.host,
		"scheme":   c.scheme,
		"baseURL":  c.baseURL,
		"isRemote": c.IsRemote(),
		"isLocal":  c.IsLocal(),
		"type":     c.GetConnectionType(),
	}
}
