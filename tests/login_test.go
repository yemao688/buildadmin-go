package tests

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// 测试前设置
func setupRouter1() *gin.Engine {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	r.POST("/login", func(c *gin.Context) {
		var json struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}

		if err := c.ShouldBindJSON(&json); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if json.Username == "admin" && json.Password == "admin123" {
			c.JSON(http.StatusOK, gin.H{"message": "Login successful"})
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		}
	})
	return r
}

// 测试 /ping 接口
func TestPingRoute(t *testing.T) {
	router := setupRouter1()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "pong", w.Body.String()[12:16]) // 假设响应格式为 {"message":"pong"}
}

// 测试 /login 接口
func TestLoginRoute(t *testing.T) {
	router := setupRouter1()

	// 成功登录
	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"username":"admin","password":"admin123"}`)
	req, _ := http.NewRequest("POST", "/login", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "Login successful", w.Body.String()[14:29]) // 假设响应格式为 {"message":"Login successful"}

	// 登录失败
	w = httptest.NewRecorder()
	body = bytes.NewBufferString(`{"username":"user","password":"pass"}`)
	req, _ = http.NewRequest("POST", "/login", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
	assert.Equal(t, "Invalid credentials", w.Body.String()[11:29]) // 假设响应格式为 {"error":"Invalid credentials"}
}

func TestLogin(t *testing.T) {
	router := setupRouter()

	// 成功登录
	w := httptest.NewRecorder()
	body := bytes.NewBufferString(``)
	req, _ := http.NewRequest("GET", "/admin/Index/login", body)
	router.ServeHTTP(w, req)
	fmt.Println(w.Body.String())
	assert.Equal(t, 200, w.Code)

	w = httptest.NewRecorder()
	body = bytes.NewBufferString(`{"username":"admin","password":"123456","keep":false,"captchaId":"a5690e15-f2a3-4f59-a9f2-1aa026ef6ed5","captchaInfo":"222,33-183,145;350;200"}`)
	req, _ = http.NewRequest("POST", "/admin/Index/login", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	fmt.Println(w.Body.String())
	assert.Equal(t, 200, w.Code)

	// assert.Equal(t, "Login successful", w.Body.String()[14:29])
}
