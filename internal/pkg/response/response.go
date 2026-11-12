// Package response 提供 admin/api 两渠道共享的 HTTP 响应封装：成功/失败
// 返回、请求事务内的暂存与提交/回滚输出。以 admin 渠道实现为基座，
// 同时提供 api 渠道的 Fail/FailByErrWithData 入口。
package response

import (
	"context"
	"database/sql/driver"
	"errors"
	"net/http"

	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/requesttx"
	"buildadmin-go/internal/pkg/util"

	"github.com/gin-gonic/gin"
	mysql "github.com/go-sql-driver/mysql"
)

type Response struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"msg"`
	Time int64       `json:"time"`
}

// 成功返回
func Success(c *gin.Context, data interface{}) {
	JsonReturn(c, http.StatusOK, 1, "", data)
}

// SuccessWithMessage returns a successful response with a translated message.
func SuccessWithMessage(c *gin.Context, message string) {
	JsonReturn(c, http.StatusOK, 1, message, nil)
}

// 返回
func JsonReturn(c *gin.Context, httpCode int, code int, msg string, data interface{}) {
	outcome := requesttx.Outcome{HTTPCode: httpCode, BusinessCode: code, Message: msg, Data: data}
	if requesttx.Active(c.Request.Context()) {
		requesttx.Stage(c.Request.Context(), outcome)
		return
	}
	writeResponse(c, outcome)
}

func writeResponse(c *gin.Context, outcome requesttx.Outcome) {
	if c.Writer.Written() {
		return
	}
	timestamp, _ := c.Get("Timestamp")
	var ts int64
	if value, ok := timestamp.(int64); ok {
		ts = value
	}
	if outcome.Message != "" {
		if _, exists := c.Get("i18n"); exists {
			outcome.Message = util.Lang(c, outcome.Message, nil)
		}
	}
	c.JSON(outcome.HTTPCode, Response{
		outcome.BusinessCode,
		outcome.Data,
		outcome.Message,
		ts,
	})
}

// CommitResponse emits the staged outcome after the request transaction has
// committed. Pass the database commit error when available; a failed commit
// emits a failure response instead of the staged success.
func CommitResponse(c *gin.Context, commitErr ...error) bool {
	if len(commitErr) > 0 && commitErr[0] != nil {
		requesttx.DiscardOutcome(c.Request.Context())
		writeError(c, commitErr[0])
		return true
	}
	outcome, ok := requesttx.TakeOutcome(c.Request.Context())
	if !ok {
		return false
	}
	writeResponse(c, outcome)
	return true
}

// RollbackResponse discards any staged outcome and emits the supplied error.
func RollbackResponse(c *gin.Context, err error) bool {
	requesttx.DiscardOutcome(c.Request.Context())
	if err == nil {
		err = cErr.BadRequest("transaction rolled back")
	}
	writeError(c, err)
	return true
}

func errorOutcome(err error) requesttx.Outcome {
	var v *cErr.Error
	if errors.As(err, &v) {
		return requesttx.Outcome{HTTPCode: v.HttpCode(), BusinessCode: v.ErrorCode(), Message: v.Error()}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, driver.ErrBadConn) {
		return requesttx.Outcome{HTTPCode: http.StatusInternalServerError, BusinessCode: cErr.ServerError, Message: "internal database error"}
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		message := "internal database error"
		if mysqlErr.Number == 1205 || mysqlErr.Number == 1213 {
			message = "database lock timeout or deadlock, please retry"
		}
		return requesttx.Outcome{HTTPCode: http.StatusInternalServerError, BusinessCode: cErr.ServerError, Message: message}
	}
	return requesttx.Outcome{HTTPCode: http.StatusBadRequest, BusinessCode: cErr.DefaultError, Message: err.Error()}
}

func writeError(c *gin.Context, err error) {
	writeResponse(c, errorOutcome(err))
}

// 失败返回
func FailByErr(c *gin.Context, err error) {
	outcome := errorOutcome(err)
	JsonReturn(c, outcome.HTTPCode, outcome.BusinessCode, outcome.Message, outcome.Data)
}

// Fail 以指定 HTTP 状态码与业务码返回失败信息（api 渠道入口）。
func Fail(c *gin.Context, httpCode int, code int, msg string) {
	JsonReturn(c, httpCode, code, msg, nil)
}

// WriteDirect 直接写出统一响应壳，跳过请求事务暂存与 msg 本地化。
//
// 供中间件使用：此时 requesttx.Active 可能为真但事务已收尾（Stage 会拒绝并
// 丢弃响应），且调用方已自行本地化 msg 或需要历史输出（time 固定 0、msg 原样）。
// handler 内请使用 Success/FailByErr，让响应与事务提交保持一致。
func WriteDirect(c *gin.Context, httpCode int, code int, msg string, data any) {
	c.JSON(httpCode, Response{code, data, msg, 0})
}

// FailByErrWithData 返回携带附加数据的失败响应（api 渠道入口）。
func FailByErrWithData(c *gin.Context, err error, data interface{}) {
	var v *cErr.Error
	if !errors.As(err, &v) {
		FailByErr(c, err)
		return
	}

	msg := util.Lang(c, v.Error(), nil)
	if requesttx.Stage(c, requesttx.Outcome{
		HTTPCode:     v.HttpCode(),
		BusinessCode: v.ErrorCode(),
		Message:      msg,
		Data:         data,
	}) {
		return
	}
	c.JSON(v.HttpCode(), Response{v.ErrorCode(), data, msg, 0})
}
