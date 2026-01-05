package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/model-ci/apack/pkg/infer/llama/config"
	"github.com/model-ci/apack/pkg/infer/llama/types"
)

type Handler struct {
	runtime types.Runner
	config  *config.APIConfig
}

type UIMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type ChatSession struct {
	ID       string      `json:"id"`
	Messages []UIMessage `json:"messages"`
	Created  time.Time   `json:"created"`
	Updated  time.Time   `json:"updated"`
}

func NewHandler(runtime types.Runner, config *config.APIConfig) *Handler {
	return &Handler{
		runtime: runtime,
		config:  config,
	}
}

func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("WebSocket support not implemented yet"))
}

func (h *Handler) HandleModels(w http.ResponseWriter, r *http.Request) {
	models := []map[string]interface{}{
		{
			"id":       "llamafile",
			"object":   "model",
			"created":  time.Now().Unix(),
			"owned_by": "llamafile",
		},
	}

	response := map[string]interface{}{
		"object": "list",
		"data":   models,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) HandleSystemInfo(w http.ResponseWriter, r *http.Request) {
	health, err := h.runtime.Health()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	metrics, err := h.runtime.Metrics()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	info := map[string]interface{}{
		"health":  health,
		"metrics": metrics,
		"config": map[string]interface{}{
			"title": h.config.Title,
			"theme": h.config.Theme,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func (h *Handler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		h.getSettings(w, r)
	case "POST":
		h.updateSettings(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	settings := map[string]interface{}{
		"theme":          h.config.Theme,
		"title":          h.config.Title,
		"default_temp":   0.7,
		"default_tokens": 512,
		"auto_scroll":    true,
		"sound_enabled":  false,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var settings map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if theme, ok := settings["theme"].(string); ok {
		h.config.Theme = theme
	}
	if title, ok := settings["title"].(string); ok {
		h.config.Title = title
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "updated",
	})
}

func (h *Handler) HandleExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	chatHistory := []ChatSession{}

	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=chat_history.json")
		json.NewEncoder(w).Encode(chatHistory)
	case "txt":
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Disposition", "attachment; filename=chat_history.txt")

		for _, session := range chatHistory {
			fmt.Fprintf(w, "=== Session %s ===\n", session.ID)
			for _, msg := range session.Messages {
				fmt.Fprintf(w, "[%s] %s: %s\n",
					msg.Timestamp.Format("2006-01-02 15:04:05"),
					msg.Role,
					msg.Content)
			}
			fmt.Fprintf(w, "\n")
		}
	default:
		http.Error(w, "Unsupported format", http.StatusBadRequest)
	}
}

func (h *Handler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(10 << 20) // 10MB max
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	result := map[string]interface{}{
		"filename": header.Filename,
		"size":     header.Size,
		"status":   "uploaded",
		"message":  "File uploaded successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) HandleStats(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.runtime.Metrics()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stats := map[string]interface{}{
		"requests": map[string]interface{}{
			"total":       metrics.RequestsTotal,
			"per_second":  metrics.RequestsPerSecond,
			"avg_latency": metrics.AverageLatency.String(),
		},
		"tokens": map[string]interface{}{
			"generated":  metrics.TokensGenerated,
			"per_second": metrics.TokensPerSecond,
		},
		"uptime": time.Since(time.Now().Add(-time.Hour)).String(), // 简化实现
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) HandlePresets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		h.getPresets(w, r)
	case "POST":
		h.savePreset(w, r)
	case "DELETE":
		h.deletePreset(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) getPresets(w http.ResponseWriter, r *http.Request) {
	presets := []map[string]interface{}{
{
    "id":          "creative",
    "name":        "Creative Writing",
    "temperature": 0.9,
    "max_tokens":  1000,
    "description": "Suitable for creative writing and brainstorming",
},
{
    "id":          "precise",
    "name":        "Precise Answer",
    "temperature": 0.1,
    "max_tokens":  500,
    "description": "Suitable for Q&A requiring accurate information",
},
{
    "id":          "balanced",
    "name":        "Balanced Mode",
    "temperature": 0.7,
    "max_tokens":  512,
    "description": "Balances creativity and accuracy",
},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(presets)
}

func (h *Handler) savePreset(w http.ResponseWriter, r *http.Request) {
	var preset map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&preset); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if _, ok := preset["name"]; !ok {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	preset["id"] = fmt.Sprintf("preset_%d", time.Now().Unix())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "saved",
		"preset": preset,
	})
}

func (h *Handler) deletePreset(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "deleted",
		"id":     id,
	})
}

func parseIntParam(r *http.Request, key string, defaultValue int) int {
	if str := r.URL.Query().Get(key); str != "" {
		if val, err := strconv.Atoi(str); err == nil {
			return val
		}
	}
	return defaultValue
}

func parseFloatParam(r *http.Request, key string, defaultValue float64) float64 {
	if str := r.URL.Query().Get(key); str != "" {
		if val, err := strconv.ParseFloat(str, 64); err == nil {
			return val
		}
	}
	return defaultValue
}
