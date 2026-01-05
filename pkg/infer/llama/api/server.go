package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/model-ci/apack/pkg/infer/llama/config"
	"github.com/model-ci/apack/pkg/infer/llama/types"
	"github.com/model-ci/apack/pkg/infer/static"
)

type Server struct {
	config     *config.APIConfig
	runtime    types.Runner
	httpServer *http.Server
	running    bool
}

func NewServer(config *config.APIConfig, runtime types.Runner) (*Server, error) {
	return &Server{
		config:  config,
		runtime: runtime,
	}, nil
}

func (s *Server) FileDump(path string) error {
	path = filepath.Dir(path)
	return fs.WalkDir(static.AssetsFiles, ".", func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		targetPath := filepath.Join(path, filePath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		content, err := static.AssetsFiles.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", filePath, err)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
		}

		if err := os.WriteFile(targetPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", targetPath, err)
		}

		return nil
	})
}

func (s *Server) Start(ctx context.Context) error {
	if s.running {
		return fmt.Errorf("view server already running")
	}

	router := httprouter.New()

	router.POST("/v1/completions", s.wrapHandler(s.handleCompletion))
	router.POST("/v1/chat/completions", s.wrapHandler(s.handleChat))
	router.POST("/v1/embeddings", s.wrapHandler(s.handleEmbedding))
	router.GET("/health", s.wrapHandler(s.handleHealth))
	router.GET("/ui/config", s.wrapHandler(s.handleAPIConfig))

	router.NotFound = s.createEnhancedStaticHandler()

	var handler http.Handler = router
	if s.config.AuthEnabled {
		handler = s.authMiddleware(handler)
	}
	handler = s.corsMiddleware(handler)
	handler = s.loggingMiddleware(handler)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("view server error: %v\n", err)
		}
	}()

	s.running = true
	return nil
}

func (s *Server) createEnhancedStaticHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		method := r.Method

		fmt.Printf("Request: %s %s\n", method, path)

		if method != "GET" {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		s.handleEmbedStatic(w, r, path)
	})
}

func (s *Server) handleEmbedStatic(w http.ResponseWriter, r *http.Request, requestPath string) {
	if requestPath == "/" {
		requestPath = "/index.html"
	}

	filePath := strings.TrimPrefix(requestPath, "/")

	if strings.Contains(filePath, "..") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	embedPath := filepath.Join("assets", filePath)

	fmt.Printf("Looking for file: %s -> %s\n", requestPath, embedPath)

	data, err := static.AssetsFiles.ReadFile(embedPath)
	if err != nil {
		if !strings.Contains(filePath, ".") {
			fmt.Printf("SPA route detected, serving index.html for: %s\n", requestPath)
			s.serveIndexFromEmbed(w, r)
			return
		}

		fmt.Printf("File not found: %s\n", embedPath)
		http.NotFound(w, r)
		return
	}

	contentType := s.getEnhancedContentType(filePath)
	w.Header().Set("Content-Type", contentType)

	s.setEnhancedCacheHeaders(w, filePath)

	s.setSecurityHeaders(w)

	if strings.HasSuffix(filePath, ".html") {
		processedHTML := s.processHTML(string(data))
		w.Write([]byte(processedHTML))
	} else {
		w.Write(data)
	}

	fmt.Printf("Served file: %s (%d bytes)\n", embedPath, len(data))
}

func (s *Server) serveIndexFromEmbed(w http.ResponseWriter, r *http.Request) {
	data, err := static.AssetsFiles.ReadFile("assets/index.html")
	if err != nil {
		fmt.Printf("Failed to read index.html: %v\n", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	processedHTML := s.processHTML(string(data))
	w.Write([]byte(processedHTML))
	fmt.Printf("Served index.html for SPA route\n")
}

func (s *Server) getEnhancedContentType(filePath string) string {
	ext := filepath.Ext(filePath)

	switch ext {
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".mjs":
		return "application/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	case ".map":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

func (s *Server) setEnhancedCacheHeaders(w http.ResponseWriter, filePath string) {
	if strings.HasSuffix(filePath, ".html") {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")

		if strings.Contains(filePath, "-") && (strings.HasSuffix(filePath, ".js") || strings.HasSuffix(filePath, ".css")) {
			w.Header().Set("Cache-Control", "public, max-age=31536000") // 1年
		}
	}
}

func (s *Server) setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
}

func (s *Server) processHTML(html string) string {
	config := fmt.Sprintf(`
        <script>
            window.types_CONFIG = {
                title: %q,
                theme: %q,
                apiBase: "/v1",
                wsBase: "/ws"
            };
        </script>
    `, s.config.Title, s.config.Theme)

	html = strings.Replace(html, "</head>", config+"</head>", 1)

	return html
}

func (s *Server) wrapHandler(h http.HandlerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		ctx := r.Context()
		for _, param := range ps {
			ctx = context.WithValue(ctx, param.Key, param.Value)
		}
		r = r.WithContext(ctx)
		h(w, r)
	}
}

func (s *Server) Stop() error {
	if !s.running {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown API server: %w", err)
	}

	s.running = false
	return nil
}

func (s *Server) IsRunning() bool {
	return s.running
}

func (s *Server) GetURL() string {
	return fmt.Sprintf("http://localhost:%d", s.config.Port)
}

func (s *Server) handleCompletion(w http.ResponseWriter, r *http.Request) {
	var req types.CompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Stream {
		s.handleCompletionStream(w, r, &req)
		return
	}

	resp, err := s.runtime.Complete(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleCompletionStream(w http.ResponseWriter, r *http.Request, req *types.CompletionRequest) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	stream, err := s.runtime.CompleteStream(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for chunk := range stream {
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req types.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Stream {
		s.handleChatStream(w, r, &req)
		return
	}

	resp, err := s.runtime.Chat(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request, req *types.ChatRequest) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	stream, err := s.runtime.ChatStream(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for chunk := range stream {
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (s *Server) handleEmbedding(w http.ResponseWriter, r *http.Request) {
	var req types.EmbedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	resp, err := s.runtime.Embed(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	m, err := s.runtime.Model(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	health := map[string]interface{}{
		"status":  "healthy",
		"api":     s.running,
		"runtime": s.runtime.IsRunning(),
		"model":   m.Data,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (s *Server) handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	config := map[string]interface{}{
		"title": s.config.Title,
		"theme": s.config.Theme,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != s.config.Username || password != s.config.Password {
			w.Header().Set("WWW-Authenticate", `Basic realm="types API"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("[API] %s %s %v\n", r.Method, r.URL.Path, time.Since(start))
	})
}
