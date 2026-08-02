package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go-build-admin/app/pkg/token"
)

func TestRegisterRefreshTypeRejectsDuplicate(t *testing.T) {
	typ := "test-refresh-duplicate"
	desc := RefreshTypeDescriptor{AccessType: "test", AccessHeader: "test-token"}
	require.Error(t, RegisterRefreshType("", desc))
	require.NoError(t, RegisterRefreshType(typ, desc))
	require.Error(t, RegisterRefreshType(typ, desc))
}

func TestRefreshTokenUsesRegisteredRefreshType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	driver := &handlerContractTokenDriver{get: &token.Token{Type: "seller-refresh", UserID: 42}}
	var gotRefreshToken string
	var gotUserID int32
	require.NoError(t, RegisterRefreshType("seller-refresh", RefreshTypeDescriptor{
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

	var response Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 1, response.Code)
	require.Equal(t, "seller-refresh", response.Data.(map[string]interface{})["type"])
	require.Equal(t, "seller-access", response.Data.(map[string]interface{})["token"])
	require.Equal(t, "seller-refresh-token", gotRefreshToken)
	require.Equal(t, int32(42), gotUserID)
	require.Zero(t, driver.setCount)
}
