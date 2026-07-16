package handler

import (
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/requesttx"
	"go-build-admin/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"msg"`
	Time int         `json:"time"`
}

// 成功返回
func Success(c *gin.Context, data interface{}) {
	if requesttx.Stage(c, requesttx.Outcome{HTTPCode: http.StatusOK, BusinessCode: 1, Message: "ok", Data: data}) {
		return
	}
	c.JSON(http.StatusOK, Response{
		1,
		data,
		"ok",
		0,
	})
}

// 失败返回
func Fail(c *gin.Context, httpCode int, code int, msg string) {
	msg = utils.Lang(c, msg, nil)
	if requesttx.Stage(c, requesttx.Outcome{HTTPCode: httpCode, BusinessCode: code, Message: msg}) {
		return
	}
	c.JSON(httpCode, Response{
		code,
		nil,
		msg,
		0,
	})
}

func FailByErr(c *gin.Context, err error) {
	v, ok := err.(*cErr.Error)
	if ok {
		Fail(c, v.HttpCode(), v.ErrorCode(), v.Error())
	} else {
		Fail(c, http.StatusBadRequest, cErr.DefaultError, err.Error())
	}
}
