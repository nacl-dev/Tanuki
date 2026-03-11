package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func streamJSON(c *gin.Context, interval time.Duration, load func() (any, error)) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		respondError(c, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	initialPayload, err := load()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	lastSnapshot, err := json.Marshal(initialPayload)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "encode stream payload: "+err.Error())
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "private, no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	if _, err := fmt.Fprintf(c.Writer, ": connected\n\ndata: %s\n\n", lastSnapshot); err != nil {
		return
	}
	flusher.Flush()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			payload, err := load()
			if err != nil {
				if _, writeErr := fmt.Fprintf(c.Writer, "event: error\ndata: %q\n\n", err.Error()); writeErr != nil {
					return
				}
				flusher.Flush()
				continue
			}

			snapshot, err := json.Marshal(payload)
			if err != nil {
				if _, writeErr := fmt.Fprintf(c.Writer, "event: error\ndata: %q\n\n", "encode stream payload"); writeErr != nil {
					return
				}
				flusher.Flush()
				continue
			}

			if bytes.Equal(snapshot, lastSnapshot) {
				if _, err := c.Writer.Write([]byte(": keep-alive\n\n")); err != nil {
					return
				}
				flusher.Flush()
				continue
			}

			lastSnapshot = snapshot
			if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", snapshot); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
