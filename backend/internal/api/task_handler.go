package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/taskqueue"
)

type TaskHandler struct {
	tasks *taskqueue.Manager
}

func newTaskHandler(tasks *taskqueue.Manager) *TaskHandler {
	return &TaskHandler{tasks: tasks}
}

func (h *TaskHandler) List(c *gin.Context) {
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	respondOK(c, h.tasks.List(limit), nil)
}

func (h *TaskHandler) Get(c *gin.Context) {
	task, ok := h.tasks.Get(c.Param("id"))
	if !ok {
		respondError(c, http.StatusNotFound, "task not found")
		return
	}
	respondOK(c, task, nil)
}
