package api

import (
	"net/http"
	"strconv"
	"time"

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
	requestedBy := c.GetString("userID")
	respondOK(c, h.tasks.ListForRequester(requestedBy, taskListLimit(c)), nil)
}

// Stream emits the current task list whenever it changes.
// GET /api/tasks/stream
func (h *TaskHandler) Stream(c *gin.Context) {
	requestedBy := c.GetString("userID")

	streamJSON(c, time.Second, func() (any, error) {
		return h.tasks.ListForRequester(requestedBy, taskListLimit(c)), nil
	})
}

func (h *TaskHandler) Get(c *gin.Context) {
	task, ok := h.tasks.GetForRequester(c.Param("id"), c.GetString("userID"))
	if !ok {
		respondError(c, http.StatusNotFound, "task not found")
		return
	}
	respondOK(c, task, nil)
}

func taskListLimit(c *gin.Context) int {
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	return limit
}
