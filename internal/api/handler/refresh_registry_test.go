package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/token"
)

func TestRefreshTokenUsesRegisteredRefreshType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	driver := &handlerContractTokenDriver{get: &token.Token{Type: "seller-refresh", UserID: 42}}
	var gotRefreshToken string
	var gotUserID int32
	require.NoError(t, token.RegisterRefreshType("seller-refresh", token.RefreshTypeDescriptor{
		AccessType:   "seller",
		AccessHeader: "seller-token",
		Refresh: func(_ *gin.Context, refreshToken string, userID int32) (string, error) {
			gotRefreshToken = refreshToken
			gotUserID = userID
			return "seller-access", nil
		},
	}))

	h := &CommonHandler{tokenHelper: &token.TokenHelper{Driver: driver}}
	router := newContractTestRouter()
	router.POST("/", h.RefreshToken)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"refreshToken":"seller-refresh-token"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("seller-token", "seller-access")
	router.ServeHTTP(recorder, request)

	var resp response.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Code)
	require.Equal(t, "seller-refresh", resp.Data.(map[string]interface{})["type"])
	require.Equal(t, "seller-access", resp.Data.(map[string]interface{})["token"])
	require.Equal(t, "seller-refresh-token", gotRefreshToken)
	require.Equal(t, int32(42), gotUserID)
	require.Zero(t, driver.setCount)
}
