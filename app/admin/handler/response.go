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
	timestamp, _ := c.Get("Timestamp")
	c.JSON(http.StatusOK, Response{
		1,
		data,
		"",
		timestamp.(int64),
	})
}

// 失败返回
func Fail(c *gin.Context, httpCode int, code int, msg string) {
	msg = utils.Lang(c, msg, nil)
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
