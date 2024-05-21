package model

import (
	"encoding/json"
	"go-build-admin/app/pkg/header"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const TableNameAdminLog = "ba_admin_log"

type AdminLog struct {
	ID         int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`                     // ID
	AdminID    int32  `gorm:"column:admin_id;not null;comment:管理员ID" json:"admin_id"`                           // 管理员ID
	Username   string `gorm:"column:username;not null;comment:管理员用户名" json:"username"`                          // 管理员用户名
	URL        string `gorm:"column:url;not null;comment:操作Url" json:"url"`                                     // 操作Url
	Title      string `gorm:"column:title;not null;comment:日志标题" json:"title"`                                  // 日志标题
	Data       string `gorm:"column:data;comment:请求数据" json:"data"`                                             // 请求数据
	IP         string `gorm:"column:ip;not null;comment:IP" json:"ip"`                                          // IP
	Useragent  string `gorm:"column:useragent;not null;comment:User-Agent" json:"useragent"`                    // User-Agent
	CreateTime int64  `gorm:"autoCreateTime;column:create_time;autoCreateTime;comment:创建时间" json:"create_time"` // 创建时间
}

func (*AdminLog) TableName() string {
	return TableNameAdminLog
}

type AdminLogModel struct {
	BaseModel
}

func NewAdminLogModel(sqlDB *gorm.DB) *AdminLogModel {
	return &AdminLogModel{
		BaseModel: BaseModel{
			TableName:        TableNameAdminLog,
			Key:              "id",
			QuickSearchField: "title",
			DataLimit:        "",
			sqlDB:            sqlDB,
		},
	}
}

func (s *AdminLogModel) List(ctx *gin.Context) (list []AdminLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.sqlDB.Table(s.TableName).Scopes(IsSuperAdmin(ctx)).Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *AdminLogModel) Add(ctx *gin.Context, params map[string]interface{}) {
	url := ctx.Request.URL.Path
	//排除一些链接
	if ok, _ := regexp.MatchString(`/^(.*)\/(select|index|logout)$/i`, url); !ok {
		return
	}

	info := header.GetAdminAuth(ctx)
	username := ""
	if info.Id != 0 {
		admin := Admin{}
		s.sqlDB.Table(TableNameAdmin).Where("id=?", info.Id).First(&admin)
		username = admin.Username
	}
	title := ctx.GetString("log_title")
	if title == "" {
		name := strings.Trim(url, "/")
		actionRule := AdminRule{}
		s.sqlDB.Table(TableNameAdminRule).Where("name=?", name).First(&actionRule)

		slashIndex := strings.LastIndex(name, "/")
		if slashIndex != -1 {
			name := name[:slashIndex]
			handerRule := AdminRule{}
			s.sqlDB.Table(TableNameAdminRule).Where("name=?", name).First(&handerRule)
			title = handerRule.Name + "-" + actionRule.Name
		}
	}

	// 对params进行脱敏处理，比如隐藏密码等敏感信息
	pattern := `/(password|salt|token)/i`
	compiledRegex := regexp.MustCompile(pattern)
	for key := range params {
		if compiledRegex.MatchString(key) {
			params[key] = "****"
		}
	}
	data, _ := json.Marshal(params)
	adminLog := AdminLog{
		AdminID:   info.Id,
		Username:  username,
		URL:       url,
		Title:     title,
		Data:      string(data),
		IP:        ctx.ClientIP(),
		Useragent: ctx.Request.UserAgent(),
	}
	s.sqlDB.Table(s.TableName).Create(&adminLog)
}

func (s *AdminLogModel) Del(ctx *gin.Context, id interface{}) error {
	err := s.sqlDB.Table(s.TableName).Where(" id=? ", id).Delete(nil).Error
	return err
}
