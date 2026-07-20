package handler

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	mysql "github.com/go-sql-driver/mysql"
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

func TestResponseMapsMySQLLockErrorsToGenericRetryableFailure(t *testing.T) {
	for _, number := range []uint16{1205, 1213} {
		t.Run(strconv.FormatUint(uint64(number), 10), func(t *testing.T) {
			c, recorder := requestWithTx()
			require.True(t, CommitResponse(c, &mysql.MySQLError{Number: number, Message: "secret database details"}))
			var response Response
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			require.Equal(t, 500, recorder.Code)
			require.Equal(t, 500, response.Code)
			require.Equal(t, "database lock timeout or deadlock, please retry", response.Msg)
			require.NotContains(t, recorder.Body.String(), "secret database details")
		})
	}
}

func TestResponseMapsOtherMySQLErrorsToGenericInternalFailure(t *testing.T) {
	c, recorder := requestWithTx()
	err := &mysql.MySQLError{Number: 1062, Message: "secret database details"}
	require.True(t, CommitResponse(c, fmt.Errorf("wrapped: %w", err)))
	require.Equal(t, 500, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "secret database details")
	require.Contains(t, recorder.Body.String(), "internal database error")
}

func TestResponseMapsCancellationAndBadConnectionToGenericInternalFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "canceled", err: fmt.Errorf("wrapped: %w", context.Canceled)},
		{name: "deadline", err: fmt.Errorf("wrapped: %w", context.DeadlineExceeded)},
		{name: "bad connection", err: fmt.Errorf("wrapped: %w", driver.ErrBadConn)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, recorder := requestWithTx()
			require.True(t, CommitResponse(c, tc.err))
			require.Equal(t, 500, recorder.Code)
			require.NotContains(t, recorder.Body.String(), tc.err.Error())
			require.Contains(t, recorder.Body.String(), "internal database error")
		})
	}
}

func TestFailByErrStagesCancellationAndMySQLFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		msg  string
	}{
		{name: "canceled", err: fmt.Errorf("wrapped: %w: secret", context.Canceled), msg: "internal database error"},
		{name: "lock timeout", err: fmt.Errorf("wrapped: %w", &mysql.MySQLError{Number: 1205, Message: "secret database details"}), msg: "database lock timeout or deadlock, please retry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, recorder := requestWithTx()
			FailByErr(c, tc.err)
			require.Equal(t, 200, recorder.Code)
			require.True(t, CommitResponse(c))
			require.Equal(t, 500, recorder.Code)
			require.Contains(t, recorder.Body.String(), tc.msg)
			require.NotContains(t, recorder.Body.String(), "secret")
		})
	}
}
