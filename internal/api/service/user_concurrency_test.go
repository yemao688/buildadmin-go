package service_test

import (
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"buildadmin-go/internal/api/service"
	commonmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/testutil"
	"buildadmin-go/internal/pkg/token"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func concurrencyTestContext() *gin.Context {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/", nil)
	return ctx
}

type refreshRaceTokenDriver struct {
	token.Driver
	getStarted chan struct{}
	once       sync.Once
}

func (d *refreshRaceTokenDriver) Get(value string) (*token.Token, error) {
	data, err := d.Driver.Get(value)
	d.once.Do(func() { close(d.getStarted) })
	return data, err
}

func TestUserTokenClearInvalidatesConcurrentRefresh(t *testing.T) {
	db, config := testutil.OpenMySQL(t)
	require.NoError(t, db.AutoMigrate(&commonmodel.User{}, &token.Token{}))

	config.Token.Algo = "sha256"
	config.Token.Key = "refresh-race-test-key"
	config.App.UserTokenKeepTime = 3600
	tokenHelper := &token.TokenHelper{Driver: &refreshRaceTokenDriver{
		Driver:     token.NewMysqlDriver(db, config),
		getStarted: make(chan struct{}),
	}}
	authModel := service.NewMemberService(db, tokenHelper, config)
	username := "refresh_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	user := commonmodel.User{Username: username, Status: "enable"}
	require.NoError(t, db.Create(&user).Error)
	const refreshToken = "refresh-race-token"
	require.NoError(t, tokenHelper.Set(refreshToken, "user-refresh", user.ID, 3600))
	t.Cleanup(func() {
		_ = tokenHelper.Delete(refreshToken)
		_ = db.Where("id = ?", user.ID).Delete(&commonmodel.User{}).Error
	})

	rowLocked := make(chan struct{})
	release := make(chan struct{})
	lockDone := make(chan error, 1)
	go func() {
		lockDone <- db.Transaction(func(tx *gorm.DB) error {
			var locked commonmodel.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", user.ID).First(&locked).Error; err != nil {
				return err
			}
			close(rowLocked)
			<-release
			return nil
		})
	}()
	<-rowLocked

	refreshDone := make(chan error, 1)
	go func() {
		_, err := authModel.RefreshUserAccessToken(refreshToken)
		refreshDone <- err
	}()
	<-tokenHelper.Driver.(*refreshRaceTokenDriver).getStarted

	// Simulate a password-change transaction clearing sessions while refresh waits
	// for the same user row lock.
	require.NoError(t, tokenHelper.Clear("user-refresh", user.ID))
	close(release)
	require.NoError(t, <-lockDone)
	require.Error(t, <-refreshDone)

	var accessTokenCount int64
	require.NoError(t, db.Model(&token.Token{}).Where("type = ? AND user_id = ?", "user", user.ID).Count(&accessTokenCount).Error)
	require.Zero(t, accessTokenCount)
}
