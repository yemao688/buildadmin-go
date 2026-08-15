package router

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// /docs 静态文档服务的门禁常量。
const (
	// docsCookieName 是密码门放行 cookie 名，值 = sha256(访问密码) 的 hex。
	docsCookieName = "ba_docs_auth"
	// docsCookieMaxAge 是放行 cookie 的存活秒数（7 天），过期后需重新输入密码。
	docsCookieMaxAge = 7 * 24 * 3600
)

// registerDocsRoute 挂载 /docs 静态文档服务（public/docs/），供业务仓库发布
// 图文操作指南等静态页面（*.html + 图片），无需改动渠道注册器即可对外提供。
//   - password 为空：公开静态挂载，语义同 /assets /static /storage；
//   - password 非空：挂载服务端密码门——未携带有效 cookie 的 GET/HEAD 返回
//     密码页，POST /docs/password 校验通过后 Set-Cookie 并回跳原路径。密码
//     可经环境变量 DOCS_PASSWORD 覆盖 YAML 配置（机密走环境，避免入库/入镜像）。
//
// 两种模式统一不展示目录列表（目录请求无 index.html 时返回 404），行为与
// /assets 等 router.Static 挂载完全一致。密码门只防随意浏览（共享密码），
// 不是安全边界；真正敏感的内容应走既有登录体系，不要用本机制保护。
func registerDocsRoute(router *gin.Engine, rootDir, password string) {
	if env := os.Getenv("DOCS_PASSWORD"); env != "" {
		password = env
	}
	docsPath := filepath.Join(rootDir, "public", "docs")
	// 精确 /docs 301 到 /docs/，与 gin Static 的精确路径重定向语义一致。
	redirectDocs := func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/docs/")
	}
	router.GET("/docs", redirectDocs)
	router.HEAD("/docs", redirectDocs)
	if password == "" {
		router.GET("/docs/*filepath", noListingStaticHandler(docsPath))
		router.HEAD("/docs/*filepath", noListingStaticHandler(docsPath))
		return
	}

	expected := docsCookieValue(password)
	// 密码提交端点不走门禁中间件；静态文件全部过门禁。
	router.POST("/docs/password", docsPasswordHandler(expected))
	router.GET("/docs/*filepath", docsGate(expected), noListingStaticHandler(docsPath))
	router.HEAD("/docs/*filepath", docsGate(expected), noListingStaticHandler(docsPath))
}

// noListingFS 禁用目录列表的 http.FileSystem（复刻 gin onlyFilesFS 语义）：
// Readdir 恒返回空，目录请求渲染不出任何条目。
type noListingFS struct {
	fs http.FileSystem
}

func (f noListingFS) Open(name string) (http.File, error) {
	file, err := f.fs.Open(name)
	if err != nil {
		return nil, err
	}
	return noListingFile{file}, nil
}

type noListingFile struct {
	http.File
}

func (f noListingFile) Readdir(int) ([]os.FileInfo, error) {
	return nil, nil
}

// noListingStaticHandler 构造 /docs 静态文件 handler：先预写 404 再交给
// http.FileServer。依赖 gin responseWriter 语义——命中文件或含 index.html
// 的目录时，FileServer 的 WriteHeader(200) 会覆盖预写的 404；纯目录请求
// 保持 404 且列表为空。行为与 gin router.Static 完全一致（不展示目录列表）。
func noListingStaticHandler(docsPath string) gin.HandlerFunc {
	fileServer := http.StripPrefix("/docs", http.FileServer(noListingFS{http.Dir(docsPath)}))
	return func(c *gin.Context) {
		c.Writer.WriteHeader(http.StatusNotFound)
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}

// docsGate 校验放行 cookie（恒定时间比较），未通过时渲染密码页；HEAD 请求
// 返回 401 空体（预取/探测不渲染页面）。只挂载在 GET/HEAD 静态路由上，
// 密码提交端点 /docs/password 不经过本中间件。
func docsGate(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cookie, err := c.Cookie(docsCookieName); err == nil &&
			subtle.ConstantTimeCompare([]byte(cookie), []byte(expected)) == 1 {
			c.Next()
			return
		}
		if c.Request.Method == http.MethodHead {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		// next 带回原请求路径（含查询串），验密后直接回跳。
		_, _ = c.Writer.WriteString(renderDocsGate(c.Request.URL.RequestURI(), false))
		c.Abort()
	}
}

// docsPasswordHandler 校验表单密码：正确则写入放行 cookie（HttpOnly）并
// 302 回跳 next，错误则重渲密码页并提示。next 只接受站内相对路径，防密码
// 页被当作开放重定向器。
func docsPasswordHandler(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		next := c.PostForm("next")
		if !validDocsNext(next) {
			next = "/docs/"
		}
		submitted := docsCookieValue(c.PostForm("password"))
		if subtle.ConstantTimeCompare([]byte(submitted), []byte(expected)) == 1 {
			c.SetCookie(docsCookieName, expected, docsCookieMaxAge, "/docs", "", false, true)
			c.Redirect(http.StatusFound, next)
			return
		}
		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_, _ = c.Writer.WriteString(renderDocsGate(next, true))
	}
}

// docsCookieValue 计算放行 cookie 值：sha256(访问密码) 的 hex。服务端只存
// 摘要并恒定时间比较，密码本体不参与任何请求往返。
func docsCookieValue(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

// validDocsNext 只接受站内相对路径（以单个 / 开头），拒绝 // 开头的协议
// 相对 URL 与任何外部地址。
func validDocsNext(next string) bool {
	return strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//")
}

// renderDocsGate 返回 /docs 密码页（内联 HTML，无前端构建依赖）。next 经
// html.EscapeString 转义后嵌入表单隐藏域，防止注入。
func renderDocsGate(next string, showError bool) string {
	errorBlock := ""
	if showError {
		errorBlock = `<p class="error">密码错误，请重试。</p>`
	}
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>文档访问</title>
<style>
  * { box-sizing: border-box; }
  body { margin: 0; min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 20px;
         background: #f3f4f6; font-family: "PingFang SC", "Helvetica Neue", "Microsoft YaHei", sans-serif; color: #1f2937; }
  .gate-card { width: min(92vw, 380px); padding: 34px 36px; border-radius: 16px; background: #fff;
               box-shadow: 0 12px 40px rgba(17, 24, 39, .12); text-align: center; }
  h1 { margin: 0 0 6px; font-size: 20px; color: #111827; font-weight: 700; }
  p { margin: 0 0 18px; color: #6b7280; font-size: 13px; }
  form { margin: 0; }
  input { width: 100%; height: 44px; padding: 0 14px; border: 1px solid #d1d5db; border-radius: 10px;
          font-size: 15px; outline: 0; }
  input:focus { border-color: #3370ff; box-shadow: 0 0 0 3px rgba(51, 112, 255, .12); }
  button { width: 100%; height: 44px; margin-top: 12px; border: 0; border-radius: 10px; background: #3370ff;
           color: #fff; font-size: 15px; font-weight: 700; cursor: pointer; }
  button:hover { background: #2a5fd9; }
  .error { margin: 10px 0 0; color: #dc2626; font-size: 12.5px; }
</style>
</head>
<body>
<main class="gate-card">
  <h1>文档访问</h1>
  <p>请输入访问密码查看操作指南</p>
  <form method="post" action="/docs/password">
    <input type="hidden" name="next" value="` + html.EscapeString(next) + `">
    <input type="password" name="password" placeholder="请输入密码" autocomplete="off" autofocus required>
    <button type="submit">进入文档</button>
    ` + errorBlock + `
  </form>
</main>
</body>
</html>
`
}
