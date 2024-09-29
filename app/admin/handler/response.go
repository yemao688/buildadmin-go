package handler

import (
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/utils"
	"net/http"

	"github.com/gin-gonic/gin"
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

// 返回
func JsonReturn(c *gin.Context, httpCode int, code int, msg string, data interface{}) {
	timestamp, _ := c.Get("Timestamp")
	msg = utils.Lang(c, msg, nil)
	c.JSON(httpCode, Response{
		code,
		data,
		msg,
		timestamp.(int64),
	})
}

// 失败返回
func FailByErr(c *gin.Context, err error) {
	v, ok := err.(*cErr.Error)
	if ok {
		JsonReturn(c, v.HttpCode(), v.ErrorCode(), v.Error(), nil)
	} else {
		JsonReturn(c, http.StatusBadRequest, cErr.DefaultError, err.Error(), nil)
	}
}
