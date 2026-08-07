package handler

// 手写请求结构数字字段宽松绑定回归：PHP 弱类型 vs Go 严格 JSON 绑定。
// 字段改为 validator.Flex* 后，字符串数字/字符串数组/字符串键映射必须
// 与数字形态等价解码；binding:"required" 语义不得回归（缺键/空值仍报错）。

import (
	dto "buildadmin-go/internal/admin/dto"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminGroupFlexBinding(t *testing.T) {
	var params AdminGroup
	require.NoError(t, json.Unmarshal([]byte(`{"pid":"2","name":"g","rules":["1","2"]}`), &params))
	require.Equal(t, int32(2), int32(params.Pid))
	require.Equal(t, []int32{1, 2}, []int32(params.Rules))

	// 混合数组：数字与字符串数字元素共存
	require.NoError(t, json.Unmarshal([]byte(`{"rules":[1,"2",3]}`), &params))
	require.Equal(t, []int32{1, 2, 3}, []int32(params.Rules))

	// 空数组/null 合法
	require.NoError(t, json.Unmarshal([]byte(`{"rules":[]}`), &params))
	require.NoError(t, json.Unmarshal([]byte(`{"rules":null}`), &params))

	// 非法元素拒绝
	require.Error(t, json.Unmarshal([]byte(`{"rules":["abc"]}`), &params))
}

func TestUserFlexBinding(t *testing.T) {
	var params User
	require.NoError(t, json.Unmarshal([]byte(`{"admin_id":"5","join_time":"1700000000"}`), &params))
	require.Equal(t, int32(5), int32(params.AdminID))
	require.Equal(t, int64(1700000000), int64(params.JoinTime))
}

func TestConfigFlexBinding(t *testing.T) {
	var params Config
	require.NoError(t, json.Unmarshal([]byte(`{"weigh":"5"}`), &params))
	require.Equal(t, int32(5), int32(params.Weigh))
}

func TestAdminNullableParentIDFlexBinding(t *testing.T) {
	var params Admin
	require.NoError(t, json.Unmarshal([]byte(`{"parent_id":"3"}`), &params))
	require.True(t, params.ParentID.IsSet)
	require.NotNil(t, params.ParentID.Value)
	require.Equal(t, int32(3), int32(*params.ParentID.Value))

	require.NoError(t, json.Unmarshal([]byte(`{"parent_id":7}`), &params))
	require.Equal(t, int32(7), int32(*params.ParentID.Value))

	require.NoError(t, json.Unmarshal([]byte(`{"parent_id":null}`), &params))
	require.True(t, params.ParentID.IsSet)
	require.Nil(t, params.ParentID.Value)
}

// TestIDSFlexBinding 验证通用 ID 结构：字符串数字 id 解码成功，
// 且 binding:"required" 语义不回归（缺 id 仍触发校验错误）。
func TestIDSFlexBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(`{"id":"7"}`))
	var params dto.IDS
	require.NoError(t, ctx.ShouldBindJSON(&params))
	require.Equal(t, int32(7), int32(params.ID))

	ctx2, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx2.Request = httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(`{"name":"x"}`))
	var missing dto.IDS
	require.Error(t, ctx2.ShouldBindJSON(&missing), "required must still reject a missing id")
}

func TestCrudUploadCompletedFlexBinding(t *testing.T) {
	var params crudUploadCompletedParams
	require.NoError(t, json.Unmarshal([]byte(`{"syncIds":{"1":"2","3":4}}`), &params))
	require.Equal(t, map[int32]int{1: 2, 3: 4}, map[int32]int(params.SyncIDs))
}
