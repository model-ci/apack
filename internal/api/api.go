package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/model-ci/apack/internal/config"
	"github.com/model-ci/apack/internal/consts"
	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/router"
	"github.com/model-ci/apack/internal/utils"
	"golang.org/x/sync/errgroup"
)

type API interface {
	Init(*config.APIConfig)
	RegisterRoutes(*router.RouterGroup)
	Unregister()
}

var apis = map[string]API{}
var apisMu sync.RWMutex

func APIRegister(name string, api API) {
	apisMu.Lock()
	defer apisMu.Unlock()
	if _, ok := apis[name]; ok {
		log.Logger.Fatalf("API %s already registered", name)
	}
	apis[name] = api
}

type Server struct {
	config      *config.APIConfig
	router      *httprouter.Router
	apis        []API
	authEnabled bool
	servers     []*http.Server
	listeners   []net.Listener
	socketPaths []string
	mu          sync.RWMutex
}

func New(c *config.APIConfig) *Server {
	return &Server{
		config:      c,
		router:      httprouter.New(),
		authEnabled: false,
	}
}

func (s *Server) SetAuthEnabled(enabled bool) {
	s.authEnabled = enabled
	log.Logger.Infof("Authentication %s", map[bool]string{true: "enabled", false: "disabled"}[enabled])
}

func (s *Server) Serve() error {
	hosts := s.config.GetHosts()
	log.Logger.Info("Starting server on multiple hosts: ", hosts)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	for _, host := range hosts {
		host := host
		g.Go(func() error {
			return s.serveOnEndpoint(ctx, host)
		})
	}

	return g.Wait()
}

func (s *Server) addServer(server *http.Server, listener net.Listener, socketPath string) *http.Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.servers = append(s.servers, server)
	if listener != nil {
		s.listeners = append(s.listeners, listener)
	}
	if socketPath != "" {
		s.socketPaths = append(s.socketPaths, socketPath)
	}
	return server
}

func (s *Server) serveOnEndpoint(ctx context.Context, endpoint string) error {
	log.Logger.Infof("Starting listener on %s", endpoint)

	if strings.HasPrefix(endpoint, consts.UnixSocketScheme) {
		return s.serveUnixSocket(ctx, endpoint)
	}

	if strings.HasPrefix(endpoint, consts.TCPScheme) {
		return s.serveTCP(ctx, endpoint)
	}

	return s.serveHTTP(ctx, endpoint)
}

func (s *Server) newHttpServer(ctx context.Context) *http.Server {
	return &http.Server{
		Handler: s.router,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
		ReadTimeout:  time.Duration(s.config.GetReadTimeout()) * time.Second,
		WriteTimeout: time.Duration(s.config.GetWriteTimeout()) * time.Second,
		IdleTimeout:  time.Duration(s.config.GetIdleTimeout()) * time.Second,
	}
}

func (s *Server) serveUnixSocket(ctx context.Context, endpoint string) error {
	socketPath := strings.TrimPrefix(endpoint, consts.UnixSocketScheme)

	if !filepath.IsAbs(socketPath) {
		socketPath = filepath.Join(s.config.GetRoot(), socketPath)
	}

	log.Logger.Debugf("unix socket: %s, %s", s.config.GetRoot(), socketPath)

	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return err
	}

	if err := os.RemoveAll(socketPath); err != nil {
		log.Logger.Warnf("Failed to remove existing socket file: %v", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on unix socket %s: %w", socketPath, err)
	}

	server := s.newHttpServer(ctx)
	s.addServer(server, listener, socketPath)

	return server.Serve(listener)
}

func (s *Server) serveTCP(ctx context.Context, endpoint string) error {
	address := strings.TrimPrefix(endpoint, consts.TCPScheme)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on TCP address %s: %w", address, err)
	}

	log.Logger.Infof("TCP server listening on %s", address)

	server := s.newHttpServer(ctx)
	s.addServer(server, listener, "")

	return server.Serve(listener)
}

func (s *Server) serveHTTP(ctx context.Context, endpoint string) error {
	server := &http.Server{
		Addr:    endpoint,
		Handler: s.router,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	s.addServer(server, nil, "")

	cert := s.config.GetCertFile()
	key := s.config.GetKeyFile()

	if s.config.GetCertFile() != "" && s.config.GetKeyFile() != "" {
		log.Logger.Info("Using HTTPS with certificate: ", cert, " and key: ", key)
		return server.ListenAndServeTLS(cert, key)
	}

	log.Logger.Warn("WARNING: Using insecure HTTP mode on ", endpoint)
	return server.ListenAndServe()
}

func (s *Server) Done() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, server := range s.servers {
		if server != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := server.Shutdown(shutdownCtx); err != nil {
				log.Logger.Warnf("Server shutdown error: %v", err)
			}
			cancel()
		}
	}

	for _, api := range s.apis {
		api.Unregister()
	}

	for _, listener := range s.listeners {
		if listener != nil {
			if err := listener.Close(); err != nil {
				log.Logger.Warnf("Listener close error: %v", err)
			}
		}
	}

	for _, socketPath := range s.socketPaths {
		if socketPath != "" {
			if err := os.RemoveAll(socketPath); err != nil {
				log.Logger.Warnf("Failed to remove socket file %s: %v", socketPath, err)
			} else {
				log.Logger.Debugf("Cleaned up socket file: %s", socketPath)
			}
		}
	}

	return nil
}

func (s *Server) Check() error {
	host, port, err := net.SplitHostPort(s.config.GetEndpoint())
	if err != nil {
		errMsg := fmt.Errorf("Invalid endpoint format: %s, endpoint: %s", err, s.config.GetEndpoint())
		log.Logger.Error(errMsg)
		return errMsg
	}

	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}

	checkAddr := net.JoinHostPort(host, port)

	for i := 0; i < 100; i++ {
		conn, err := net.DialTimeout("tcp", checkAddr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			log.Logger.Info("Server is ready", "endpoint", s.config.GetEndpoint())
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	log.Logger.Warn("Server ready check timeout, but proceeding anyway", "endpoint", s.config.GetEndpoint())
	return nil
}

func (s *Server) Init() error {
	if len(apis) == 0 {
		return fmt.Errorf("no APIs registered")
	}

	for _, cpo := range s.config.Components {
		if modules, ok := s.config.ComponentModules[cpo]; ok {
			for _, mod := range modules {
				if api, ok := apis[mod]; ok {
					s.apis = append(s.apis, api)
				}
			}
		}
	}

	if len(s.apis) <= 0 {
		return fmt.Errorf("no APIs registered")
	}

	for _, api := range s.apis {
		api.Init(s.config)
		api.RegisterRoutes(router.New(s.router))
	}

	handler := s.buildHandler()
	for i := 0; i < len(s.servers); i++ {
		s.servers[i].Handler = handler
	}

	return nil
}

func (s *Server) buildHandler() http.Handler {
	return s.panicRecovery(
		s.logging(
			s.cors(
				s.validation(
					s.auth(s.router)))))
}

func (s *Server) panicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Logger.Errorf("Recovered from panic: %v\n%s", rec, debug.Stack())
				utils.WriteError(w, "INTERNAL_ERROR", "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Logger.Debugf("%s %s %v [%s]",
			r.Method,
			r.URL.Path,
			time.Since(start),
			r.RemoteAddr)
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, HEAD, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Docker-Content-Digest, Content-Range")
		w.Header().Set("Access-Control-Expose-Headers", "Docker-Content-Digest, Docker-Distribution-API-Version, Location, Range")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) validation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := ExtractPathParams(r)
		if name := params["name"]; name != "" {
			if !utils.ValidName(name) {
				utils.WriteError(w, "NAME_INVALID", "Invalid repository name", http.StatusBadRequest)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")

		if !s.authEnabled {
			next.ServeHTTP(w, r)
			return
		}

		if !strings.HasPrefix(r.URL.Path, "/v2/") {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="/auth",service="registry"`)
			utils.WriteError(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			utils.WriteError(w, "UNAUTHORIZED", "invalid credentials", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func ExtractPathParams(r *http.Request) map[string]string {
	params := make(map[string]string)
	return params
}
