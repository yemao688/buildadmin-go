package clickcaptcha

import (
	"buildadmin-go/internal/pkg/captcha"
	"buildadmin-go/internal/pkg/util"
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/magiconair/properties/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestRandPosition(t *testing.T) {

	pointArr := []*Point{
		{Size: 21, Icon: true, Name: "bicycle", Text: "<自行车>", Width: 32, Height: 32, X: 0, Y: 0},
		{Size: 18, Icon: true, Name: "wolf head", Text: "<狼头>", Width: 32, Height: 32, X: 0, Y: 0},
		{Size: 27, Icon: true, Name: "bomb", Text: "<炸弹>", Width: 32, Height: 32, X: 0, Y: 0},
		// {Size: 27, Icon: true, Name: "bomb", Text: "<炸弹>", Width: 32, Height: 32, X: 0, Y: 0},
		{Size: 28, Icon: false, Name: "", Text: "三", Width: 32, Height: 32, X: 0, Y: 0},
	}
	clickCaptcha := ClickCaptcha{}
	for _, v := range pointArr {
		v.X, v.Y = clickCaptcha.RandPosition(pointArr, 350, 200, v.Height, v.Height, v.Icon)
	}
	for _, v := range pointArr {
		assert.Equal(t, true, v.X >= 0 && v.X <= 350, "x 位置超出范围")
		assert.Equal(t, true, v.Y >= 0 && v.Y <= 350, "y 位置超出范围")
	}
}

func newCheckTestCaptcha(t *testing.T, pointCount int) *ClickCaptcha {	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:clickcaptcha-"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE ba_captcha (
		key TEXT PRIMARY KEY,
		code TEXT,
		captcha TEXT,
		create_time INTEGER,
		expire_time INTEGER
	)`).Error; err != nil {
		t.Fatal(err)
	}

	points := make([]*Point, pointCount)
	for i := range points {
		points[i] = &Point{Width: 30, Height: 30, X: 10, Y: 20}
	}
	captchaJSON, err := json.Marshal(CaptchaInfo{Width: 350, Height: 200, PointArr: points})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&captcha.Captcha{
		Key:        util.Md5("clickcaptcha-test"),
		Captcha:    string(captchaJSON),
		ExpireTime: time.Now().Unix() + 600,
	}).Error; err != nil {
		t.Fatal(err)
	}

	return &ClickCaptcha{sqlDB: db}
}

func checkWithoutPanic(t *testing.T, clickCaptcha *ClickCaptcha, info string) bool {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Check panicked for %q: %v", info, recovered)
		}
	}()
	return clickCaptcha.Check("clickcaptcha-test", info, false)
}

func TestCheckRejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name        string
		info        string
		pointCount  int
		wantSuccess bool
	}{
		{name: "empty input", info: "", pointCount: 1},
		{name: "truncated info array", info: "15,10", pointCount: 1},
		{name: "non numeric x coordinate", info: "not-a-number,10;350;200", pointCount: 1},
		{name: "non numeric y coordinate", info: "15,not-a-number;350;200", pointCount: 1},
		{name: "zero x divisor", info: "15,10;1;200", pointCount: 1},
		{name: "zero y divisor", info: "15,10;350;1", pointCount: 1},
		{name: "fewer points than expected", info: "15,10;350;200", pointCount: 2},
		{name: "well formed control", info: "15,10;350;200", pointCount: 1, wantSuccess: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clickCaptcha := newCheckTestCaptcha(t, test.pointCount)
			if got := checkWithoutPanic(t, clickCaptcha, test.info); got != test.wantSuccess {
				t.Fatalf("Check(%q) = %v, want %v", test.info, got, test.wantSuccess)
			}
		})
	}
}

func TestCheckMalformedInputNeverPanics(t *testing.T) {
	clickCaptcha := newCheckTestCaptcha(t, 1)
	rng := rand.New(rand.NewSource(1))

	for i := 0; i < 1000; i++ {
		garbage := make([]byte, rng.Intn(64))
		for j := range garbage {
			garbage[j] = byte(rng.Intn(256))
		}
		checkWithoutPanic(t, clickCaptcha, string(garbage))
	}
}

// TestNilDBNeverPanics: 安装向导模式下 DB 连接为 nil（应用以只读基座启动），
// 点击验证码必须报错/失败而不是空指针 panic。
func TestNilDBNeverPanics(t *testing.T) {
	clickCaptcha := &ClickCaptcha{}

	if _, err := clickCaptcha.Create(nil, "some-id"); err == nil {
		t.Fatal("Create with nil sqlDB should return an error")
	}
	if clickCaptcha.Check("some-id", "", false) {
		t.Fatal("Check with nil sqlDB should fail")
	}
}
