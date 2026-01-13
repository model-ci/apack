package base

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/model-ci/apack/internal/api"
	"github.com/model-ci/apack/internal/config"
	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/router"
	"github.com/model-ci/apack/internal/service"
	"github.com/model-ci/apack/internal/task"
	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/internal/utils"
)

const (
	Namespace = "/base"
)

func init() {
	API := BaseAPI{namespace: Namespace}
	api.APIRegister(API.namespace, &API)
}

type BaseAPI struct {
	namespace string
	service   *service.BaseService
	manager   *task.Manager
	router    *router.RouterGroup
	mu        sync.RWMutex
}

func (b *BaseAPI) Init(c *config.APIConfig) {
	b.manager = task.NewManager(3 * time.Second)
	s, err := service.NewBaseService(c.GetDataDir(), b.manager,
		&service.Config{Concurrency: c.Base.Opts.Concurrency, Compress: c.Base.Opts.Compress})
	if err != nil {
		panic(err)
	}
	b.service = s
}

func (b *BaseAPI) RegisterRoutes(rr *router.RouterGroup) {
	log.Logger.Info("Registering Base API routes")

	ns := rr.Group(b.namespace)

	ns.POST("/v1/gen", b.handleGen)
	ns.POST("/v1/pull", b.handlePull)
	ns.POST("/v1/push", b.handlePush)
	ns.POST("/v1/build", b.handleBuild)
	ns.POST("/v1/tag", b.handleTag)
	ns.POST("/v1/export", b.handleExport)
	ns.POST("/v1/import", b.handleImport)
	ns.POST("/v1/list", b.handleList)
	ns.POST("/v1/info", b.handleInfo)
	ns.POST("/v1/inspect", b.handleInspect)
	ns.POST("/v1/remove", b.handleRemove)

	ns.POST("/v1/run", b.handleRun)
	ns.POST("/v1/ps", b.handlePs)
	ns.POST("/v1/kill", b.handleKill)

	ns.GET("/v1/tasks/:taskId", b.handleGetTask)
	ns.GET("/v1/tasks/:taskId/stream", b.handleStreamProgress)
	ns.POST("/v1/tasks/:taskId/cancel", b.handleCancelTask)
	ns.GET("/v1/tasks", b.handleListTasks)

	ns.GET("/health", b.handleHealth)
	ns.GET("/ready", b.handleReady)
}

func (b *BaseAPI) Unregister() {

}

func (b *BaseAPI) handlePush(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	push := &types.Request{}
	if err := push.Decode(req.Body); err != nil {
		utils.WriteError(w, "PUSH", "failed to read data", http.StatusInternalServerError)
		return
	}

	res, err := b.service.Push(req.Context(), push)
	if err != nil {
		utils.WriteError(w, "PUSH", "push error", http.StatusInternalServerError)
		return
	}

	utils.WriteJSON(w, res, http.StatusOK)
}

func (b *BaseAPI) handleGen(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	gen := &types.Request{}
	if err := gen.Decode(req.Body); err != nil {
		utils.WriteError(w, "GEN", "failed to read data", http.StatusInternalServerError)
		return
	}

	res, err := b.service.Gen(req.Context(), gen)
	if err != nil {
		utils.WriteError(w, "GEN", err.Error(), http.StatusInternalServerError)
		return
	}

	utils.WriteJSON(w, res, http.StatusOK)
}

func (b *BaseAPI) handlePull(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	pull := &types.Request{}
	if err := pull.Decode(req.Body); err != nil {
		utils.WriteError(w, "PULL", "failed to read data", http.StatusInternalServerError)
		return
	}

	// TODO: check pull

	res, err := b.service.Pull(req.Context(), pull)
	if err != nil {
		utils.WriteError(w, "PULL", "pull error", http.StatusInternalServerError)
		return
	}

	utils.WriteJSON(w, res, http.StatusOK)
}

func (b *BaseAPI) handleList(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	list := &types.Request{}
	if err := list.Decode(req.Body); err != nil {
		utils.WriteError(w, "LIST", "failed to read data", http.StatusInternalServerError)
		return
	}

	res, err := b.service.List(req.Context(), list)
	if err != nil {
		utils.WriteError(w, "LIST", err.Error(), http.StatusInternalServerError)
		return
	}

	utils.WriteJSON(w, res, http.StatusOK)
}

func (b *BaseAPI) handleInfo(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	info := &types.Request{}
	if err := info.Decode(req.Body); err != nil {
		utils.WriteError(w, "INFO", "failed to read data", http.StatusInternalServerError)
		return
	}

	res, err := b.service.Info(req.Context(), info)
	if err != nil {
		utils.WriteError(w, "INFO", "info error", http.StatusInternalServerError)
		return
	}

	utils.WriteJSON(w, res, http.StatusOK)
}

func (b *BaseAPI) handleInspect(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	inspect := &types.Request{}
	if err := inspect.Decode(req.Body); err != nil {
		utils.WriteError(w, "INSPECT", "failed to read data", http.StatusInternalServerError)
		return
	}

	res, err := b.service.Inspect(req.Context(), inspect)
	if err != nil {
		utils.WriteError(w, "INSPECT", "inspect error", http.StatusInternalServerError)
		return
	}

	utils.WriteJSON(w, res, http.StatusOK)
}

func (b *BaseAPI) handleRemove(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	remove := &types.Request{}
	if err := remove.Decode(req.Body); err != nil {
		utils.WriteError(w, "REMOVE", "failed to read data", http.StatusInternalServerError)
		return
	}

	res, err := b.service.Remove(req.Context(), remove)
	if err != nil {
		utils.WriteError(w, "REMOVE", "remove error", http.StatusInternalServerError)
		return
	}

	utils.WriteJSON(w, res, http.StatusOK)
}

func (b *BaseAPI) handleRun(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	run := &types.Request{}
	if err := run.Decode(req.Body); err != nil {
		utils.WriteError(w, "RUN", "failed to read data", http.StatusInternalServerError)
		return
	}

	res, err := b.service.Run(req.Context(), run)
	if err != nil {
		utils.WriteError(w, "RUN", "run error", http.StatusInternalServerError)
		return
	}

	utils.WriteJSON(w, res, http.StatusOK)
}

func (b *BaseAPI) handlePs(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	ps := &types.Request{}
	if err := ps.Decode(req.Body); err != nil {
		utils.WriteError(w, "PS", "failed to read data", http.StatusInternalServerError)
		return
	}

	res, err := b.service.Ps(req.Context(), ps)
	if err != nil {
		utils.WriteError(w, "PS", err.Error(), http.StatusInternalServerError)
		return
	}

	utils.WriteJSON(w, res, http.StatusOK)
}

func (b *BaseAPI) handleKill(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	kill := &types.Request{}
	if err := kill.Decode(req.Body); err != nil {
		utils.WriteError(w, "KILL", "failed to read data", http.StatusInternalServerError)
		return
	}

	res, err := b.service.Kill(req.Context(), kill)
	if err != nil {
		utils.WriteError(w, "KILL", "kill error", http.StatusInternalServerError)
		return
	}

	utils.WriteJSON(w, res, http.StatusOK)
}

func (b *BaseAPI) handleBuild(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	build := &types.Request{}
	if err := build.Decode(req.Body); err != nil {
		utils.WriteError(w, "BUILD", "failed to read data", http.StatusInternalServerError)
		return
	}

	res, err := b.service.Build(req.Context(), build)
	if err != nil {
		utils.WriteError(w, "BUILD", err.Error(), http.StatusInternalServerError)
		return
	}

	utils.WriteJSON(w, res, http.StatusOK)
}

func (b *BaseAPI) handleTag(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	tag := &types.Request{}
	if err := tag.Decode(req.Body); err != nil {
		utils.WriteError(w, "TAG", "failed to read data", http.StatusInternalServerError)
		return
	}

	if tag.Reference.String() == "" || tag.ReferenceStr == "" {
		utils.WriteError(w, "TAG", "reference or reference_str is nil", http.StatusInternalServerError)
		return
	}

	if tag.Params.TargetRef.String() == "" {
		utils.WriteError(w, "TAG", "target_ref is nil", http.StatusInternalServerError)
		return
	}

	res, err := b.service.Tag(req.Context(), tag)
	if err != nil {
		utils.WriteError(w, "TAG", err.Error(), http.StatusInternalServerError)
		return
	}

	utils.WriteJSON(w, res, http.StatusOK)
}

func (b *BaseAPI) handleExport(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	export := &types.Request{}
	if err := export.Decode(req.Body); err != nil {
		utils.WriteError(w, "EXPORT", "failed to read data", http.StatusInternalServerError)
		return
	}

	res, err := b.service.Export(req.Context(), export)
	if err != nil {
		utils.WriteError(w, "EXPORT", "export error", http.StatusInternalServerError)
		return
	}

	utils.WriteJSON(w, res, http.StatusOK)
}

func (b *BaseAPI) handleImport(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	export := &types.Request{}
	if err := export.Decode(req.Body); err != nil {
		utils.WriteError(w, "IMPORT", "failed to read data", http.StatusInternalServerError)
		return
	}

	res, err := b.service.Import(req.Context(), export)
	if err != nil {
		utils.WriteError(w, "IMPORT", "import error", http.StatusInternalServerError)
		return
	}

	utils.WriteJSON(w, res, http.StatusOK)
}

func (b *BaseAPI) handleGetTask(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	taskID := ps.ByName("taskId")
	if taskID == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	task, exists := b.manager.GetTask(taskID)
	if !exists {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	utils.WriteJSON(w, task, http.StatusOK)
}

func (b *BaseAPI) handleStreamProgress(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	taskID := ps.ByName("taskId")
	if taskID == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionContextTakeover,
		OriginPatterns:  []string{"*"},
	})
	if err != nil {
		log.Logger.Errorf("Failed to upgrade websocket: %v", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "internal error")

	ctx := r.Context()

	eventCh := make(chan task.Event, 100)
	defer close(eventCh)

	if err := b.manager.Subscribe(taskID, eventCh); err != nil {
		conn.Close(websocket.StatusUnsupportedData, err.Error())
		return
	}
	defer b.manager.Unsubscribe(taskID, eventCh)

	if t, exists := b.manager.GetTask(taskID); exists {
		initialEvent := task.Event{
			TaskID:    taskID,
			Type:      task.EventProgress,
			Progress:  t.Progress,
			Total:     t.Total,
			Timestamp: time.Now(),
		}
		b.writeEvent(conn, &initialEvent)
	}

	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				conn.Close(websocket.StatusNormalClosure, "task completed")
				return
			}

			if err := b.writeEvent(conn, &event); err != nil {
				log.Logger.Errorf("Failed to write event: %v", err)
				return
			}

			if event.Type == task.EventComplete || event.Type == task.EventError || event.Type == task.EventCancel {
				conn.Close(websocket.StatusNormalClosure, "task finished")
				return
			}

		case <-ctx.Done():
			conn.Close(websocket.StatusGoingAway, "client disconnected")
			return
		}
	}
}

func (b *BaseAPI) writeEvent(conn *websocket.Conn, event *task.Event) error {
	data, err := event.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	return conn.Write(context.Background(), websocket.MessageText, data)
}

func (b *BaseAPI) handleCancelTask(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	taskID := ps.ByName("taskId")
	if taskID == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	task, exists := b.manager.GetTask(taskID)
	if !exists {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	task.Cancel()
	b.manager.CompleteTask(taskID, fmt.Errorf("task canceled by user"))

	w.WriteHeader(http.StatusOK)
}

func (b *BaseAPI) handleListTasks(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tasks := b.manager.ListTasks()

	items := &task.Items{
		Tasks: tasks,
		Count: len(tasks),
	}

	utils.WriteJSON(w, items, http.StatusOK)
}

func (b *BaseAPI) handleHealth(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	hs := &types.HealthStatus{
		Status: "healthy",
	}
	utils.WriteStatusJSON(w, hs, http.StatusOK)
}

func (b *BaseAPI) handleReady(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	hs := &types.HealthStatus{
		Status: "ready",
	}
	utils.WriteStatusJSON(w, hs, http.StatusOK)
}

func API(path string) string {
	return Namespace + path
}
