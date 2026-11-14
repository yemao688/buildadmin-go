package route

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func okHandler(c *gin.Context) {
	c.Status(http.StatusOK)
}

//nolint:paralleltest // 测试改写包级路由快照 registeredRoutes，并行会污染同包其它用例
func TestCollectRoutesSnapshotsMethodPathAndHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/admin/index/index", okHandler)
	engine.POST("/admin/auth/login", okHandler)

	CollectRoutes(engine)

	got := GetAllRoutes()
	require.Len(t, got, 2)
	snapshot := make(map[string]string, len(got))
	for _, item := range got {
		snapshot[item.Method+" "+item.Path] = item.Handler
	}
	require.Contains(t, snapshot, "GET /admin/index/index")
	require.Contains(t, snapshot, "POST /admin/auth/login")
	require.Contains(t, snapshot["GET /admin/index/index"], "okHandler", "快照应保留处理器名")
}

//nolint:paralleltest // 测试改写包级路由快照 registeredRoutes，并行会污染同包其它用例
func TestCollectRoutesReplacesPreviousSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	first := gin.New()
	first.GET("/first", okHandler)
	CollectRoutes(first)
	require.Len(t, GetAllRoutes(), 1)

	second := gin.New()
	second.GET("/second", okHandler)
	second.PUT("/third", okHandler)
	CollectRoutes(second)

	got := GetAllRoutes()
	require.Len(t, got, 2)
	for _, item := range got {
		require.NotEqual(t, "/first", item.Path, "第二次收集应替换上一次快照")
	}
}

//nolint:paralleltest // 测试改写包级路由快照 registeredRoutes，并行会污染同包其它用例
func TestCollectRoutesEmptyEngineYieldsEmptySnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	seeded := gin.New()
	seeded.GET("/seeded", okHandler)
	CollectRoutes(seeded)
	require.NotEmpty(t, GetAllRoutes())

	CollectRoutes(gin.New())
	require.Empty(t, GetAllRoutes(), "空引擎应产生空快照而不是保留旧快照")
}
