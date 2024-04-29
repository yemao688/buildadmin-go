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
	Time int         `json:"time"`
}

// 成功返回
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		0,
		data,
		"ok",
		0,
	})
}

// 失败返回
func Fail(c *gin.Context, httpCode int, code int, msg string) {
	msg = utils.Lange(c, msg, nil)
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

func FailByErrTemp(c *gin.Context, err error, templateData map[string]string) {
	v, ok := err.(*cErr.Error)
	if ok {
		msg := utils.Lange(c, v.Error(), templateData)
		c.JSON(v.HttpCode(), Response{
			v.ErrorCode(),
			nil,
			msg,
			0,
		})
	} else {
		msg := utils.Lange(c, err.Error(), templateData)
		c.JSON(http.StatusBadRequest, Response{
			cErr.DefaultError,
			nil,
			msg,
			0,
		})
	}
}
