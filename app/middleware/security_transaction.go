package middleware

import (
	"bytes"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go-build-admin/app/pkg/requesttx"
	"gorm.io/gorm"
)

var errBusinessRollback = errors.New("requesttx: business response requested rollback")
var errMissingOutcome = errors.New("requesttx: protected handler produced no staged outcome")
var errDirectResponse = errors.New("requesttx: direct response bypassed staging")

type transactionResponseWriter struct {
	gin.ResponseWriter
	status  int
	started bool
	body    bytes.Buffer
	header  http.Header
}

func (w *transactionResponseWriter) WriteHeader(code int) {
	if !w.started && code > 0 {
		w.status = code
	}
}

func (w *transactionResponseWriter) WriteHeaderNow() {
	if !w.started {
		w.started = true
	}
}

func (w *transactionResponseWriter) Header() http.Header { return w.header }

func (w *transactionResponseWriter) Write(p []byte) (int, error) {
	w.WriteHeaderNow()
	return w.body.Write(p)
}

func (w *transactionResponseWriter) WriteString(s string) (int, error) {
	w.WriteHeaderNow()
	return w.body.WriteString(s)
}

func (w *transactionResponseWriter) Status() int   { return w.status }
func (w *transactionResponseWriter) Size() int     { return w.body.Len() }
func (w *transactionResponseWriter) Written() bool { return w.started }

func (w *transactionResponseWriter) flush() {
	for key, values := range w.header {
		w.ResponseWriter.Header()[key] = append([]string(nil), values...)
	}
	if !w.started {
		return
	}
	w.ResponseWriter.WriteHeader(w.status)
	if w.body.Len() > 0 {
		_, _ = w.ResponseWriter.Write(w.body.Bytes())
	}
}

func (m *Security) runRequestTransaction(c *gin.Context) {
	var outcome requesttx.Outcome
	var hasOutcome bool
	var txErr error
	originalWriter := c.Writer
	bufferedWriter := &transactionResponseWriter{ResponseWriter: originalWriter, status: http.StatusOK, header: make(http.Header)}
	c.Writer = bufferedWriter
	finishRequest := func() {
		if c.Request == nil {
			return
		}
		boundContext := c.Request.Context()
		requesttx.Finish(boundContext)
		c.Request = c.Request.WithContext(requesttx.Unbind(boundContext))
	}
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				requesttx.DiscardOutcome(c.Request.Context())
				finishRequest()
				c.Writer = originalWriter
				panic(recovered)
			}
		}()
		txErr = m.sqlDB.Transaction(func(tx *gorm.DB) error {
			bound := requesttx.Bind(c.Request.Context(), tx.WithContext(c.Request.Context()))
			c.Request = c.Request.WithContext(bound)
			m.workHandler()(c)
			outcome, hasOutcome = requesttx.PeekOutcome(c.Request.Context())
			if !hasOutcome {
				return errMissingOutcome
			}
			if bufferedWriter.Written() {
				return errDirectResponse
			}
			if outcome.BusinessCode != 1 {
				return errBusinessRollback
			}
			if c.IsAborted() || c.Writer.Status() >= http.StatusBadRequest {
				return errors.New("requesttx: request aborted")
			}
			return nil
		})
	}()
	if txErr != nil {
		if errors.Is(txErr, errBusinessRollback) {
			if out, ok := requesttx.TakeOutcome(c.Request.Context()); ok {
				c.Writer = originalWriter
				c.JSON(out.HTTPCode, gin.H{"code": out.BusinessCode, "data": out.Data, "msg": out.Message, "time": 0})
			}
			finishRequest()
			return
		}
		requesttx.DiscardOutcome(c.Request.Context())
		finishRequest()
		if bufferedWriter.Written() {
			m.log.Warn("[ DataSecurity ] direct response discarded after transaction failure")
		}
		c.Writer = originalWriter
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "data": nil, "msg": "transaction failed", "time": 0})
		return
	}
	c.Writer = originalWriter
	requesttx.RunAfterCommit(c.Request.Context())
	bufferedWriter.flush()
	if out, ok := requesttx.TakeOutcome(c.Request.Context()); ok {
		c.JSON(out.HTTPCode, gin.H{"code": out.BusinessCode, "data": out.Data, "msg": out.Message, "time": 0})
	}
	finishRequest()
}
