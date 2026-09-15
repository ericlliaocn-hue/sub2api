package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// WatchedTrafficMiddleware records prompt + upstream response for a short
// explicit user list. When the kill switch is off, it is a single atomic
// load and return — no body tee, no writer wrap, no external API.
func WatchedTrafficMiddleware(svc *service.WatchedTrafficService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if svc == nil || !svc.Enabled() {
			c.Next()
			return
		}
		apiKey, ok := middleware2.GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || !svc.ShouldWatch(apiKey.UserID) {
			c.Next()
			return
		}
		capture := attachWatchedCapture(c)
		c.Next()
		svc.Enqueue(capture.record(c, apiKey))
	}
}

type watchedCapture struct {
	body   *limitedCaptureReader
	writer *watchedCaptureWriter
}

func attachWatchedCapture(c *gin.Context) *watchedCapture {
	cap := &watchedCapture{}
	if c.Request != nil && c.Request.Body != nil && c.Request.Body != http.NoBody {
		cap.body = &limitedCaptureReader{inner: c.Request.Body, limit: service.WatchedTrafficTextLimit}
		c.Request.Body = cap.body
	}
	if c.Writer != nil {
		cap.writer = &watchedCaptureWriter{ResponseWriter: c.Writer, limit: service.WatchedTrafficTextLimit}
		c.Writer = cap.writer
	}
	return cap
}

func (cap *watchedCapture) record(c *gin.Context, apiKey *service.APIKey) service.WatchedTrafficRecord {
	rec := service.WatchedTrafficRecord{
		RequestID: watchedRequestID(c),
		Endpoint:  watchedEndpoint(c),
	}
	if apiKey != nil {
		rec.UserID = apiKey.UserID
		rec.APIKeyID = optionalPositiveInt64(apiKey.ID)
		rec.APIKeyName = apiKey.Name
		if apiKey.User != nil {
			rec.UserEmail = apiKey.User.Email
		}
		if apiKey.GroupID != nil {
			rec.GroupID = optionalPositiveInt64(*apiKey.GroupID)
		}
		if apiKey.Group != nil {
			rec.GroupName = apiKey.Group.Name
			if rec.GroupID == nil {
				rec.GroupID = optionalPositiveInt64(apiKey.Group.ID)
			}
		}
	}
	if cap != nil && cap.body != nil {
		rec.Model, rec.PromptText = service.ExtractWatchedPrompt(cap.body.bytes())
	}
	if rec.Model == "" {
		if model, ok := c.Get(opsModelKey); ok {
			if name, ok := model.(string); ok {
				rec.Model = name
			}
		}
	}
	if rec.Model == "" {
		if model, ok := c.Request.Context().Value(ctxkey.Model).(string); ok {
			rec.Model = model
		}
	}
	if accountID, ok := c.Request.Context().Value(ctxkey.AccountID).(int64); ok {
		rec.AccountID = optionalPositiveInt64(accountID)
	}
	if cap != nil && cap.writer != nil {
		rec.StatusCode = cap.writer.Status()
		rec.ResponseText, rec.ErrorText = service.ExtractWatchedResponse(cap.writer.bytes(), rec.StatusCode)
	} else {
		rec.StatusCode = c.Writer.Status()
	}
	return rec
}

func watchedRequestID(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	if id, ok := c.Request.Context().Value(ctxkey.RequestID).(string); ok && strings.TrimSpace(id) != "" {
		return id
	}
	if id, ok := c.Request.Context().Value(ctxkey.ClientRequestID).(string); ok {
		return id
	}
	return ""
}

func optionalPositiveInt64(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	return &v
}

func watchedEndpoint(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	if path := strings.TrimSpace(c.FullPath()); path != "" {
		return c.Request.Method + " " + path
	}
	return c.Request.Method + " " + c.Request.URL.Path
}

type limitedCaptureReader struct {
	inner io.ReadCloser
	buf   []byte
	limit int
}

func (r *limitedCaptureReader) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	if n > 0 && len(r.buf) < r.limit {
		take := n
		if remain := r.limit - len(r.buf); take > remain {
			take = remain
		}
		r.buf = append(r.buf, p[:take]...)
	}
	return n, err
}

func (r *limitedCaptureReader) Close() error {
	if r.inner == nil {
		return nil
	}
	return r.inner.Close()
}

func (r *limitedCaptureReader) bytes() []byte {
	if r == nil {
		return nil
	}
	return r.buf
}

type watchedCaptureWriter struct {
	gin.ResponseWriter
	buf   []byte
	limit int
}

func (w *watchedCaptureWriter) Write(data []byte) (int, error) {
	w.capture(data)
	return w.ResponseWriter.Write(data)
}

func (w *watchedCaptureWriter) WriteString(s string) (int, error) {
	w.capture([]byte(s))
	if ws, ok := w.ResponseWriter.(interface{ WriteString(string) (int, error) }); ok {
		return ws.WriteString(s)
	}
	return w.ResponseWriter.Write([]byte(s))
}

func (w *watchedCaptureWriter) capture(data []byte) {
	if w == nil || len(data) == 0 || len(w.buf) >= w.limit {
		return
	}
	take := len(data)
	if remain := w.limit - len(w.buf); take > remain {
		take = remain
	}
	w.buf = append(w.buf, data[:take]...)
}

func (w *watchedCaptureWriter) bytes() []byte {
	if w == nil {
		return nil
	}
	return w.buf
}
