package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go-build-admin/app/pkg/requesttx"
	"gorm.io/gorm"
)

func requestWithTx() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest("POST", "/", nil)
	request = request.WithContext(requesttx.Bind(context.Background(), &gorm.DB{}))
	c.Request = request
	c.Set("Timestamp", int64(1))
	return c, recorder
}

func TestResponseStagesAndCommitsOnce(t *testing.T) {
	c, recorder := requestWithTx()
	Success(c, map[string]string{"state": "ok"})
	require.Equal(t, 200, recorder.Code)
	require.True(t, CommitResponse(c))
	require.Equal(t, 200, recorder.Code)
	require.False(t, CommitResponse(c))
	var response Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 1, response.Code)
}

func TestResponseCommitFailureOutputsFailure(t *testing.T) {
	c, recorder := requestWithTx()
	Success(c, "uncommitted")
	require.True(t, CommitResponse(c, errors.New("commit failed")))
	var response Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 400, response.Code)
}

func TestResponseRollbackOutputsFailure(t *testing.T) {
	c, recorder := requestWithTx()
	Success(c, "discarded")
	require.True(t, RollbackResponse(c, errors.New("rolled back")))
	require.Contains(t, recorder.Body.String(), "rolled back")
}
