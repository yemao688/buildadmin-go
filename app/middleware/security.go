package middleware

import (
	"encoding/json"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/header"
	"go-build-admin/conf"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Security struct {
	config *conf.Configuration
	log    *zap.Logger
	sqlDB  *gorm.DB
}

func NewSecurity(
	config *conf.Configuration,
	log *zap.Logger,
	sqlDB *gorm.DB,
) *Security {
	return &Security{
		config: config,
		log:    log,
		sqlDB:  sqlDB,
	}
}

func (m *Security) Handler() gin.HandlerFunc {

	return func(c *gin.Context) {

		//记录删除数据操作
		if c.Request.Method == http.MethodDelete {
			path := c.FullPath()
			recycle := model.SecurityDataRecycle{}
			err := m.sqlDB.Model(&model.SecurityDataRecycle{}).Where(" status=? and controller_as=?", "1", path).First(&recycle).Error
			if err != nil {
				return
			}

			rows := []map[string]any{}
			err = m.sqlDB.Table(m.config.Database.Prefix+recycle.DataTable).Where(" ids in ? ", []string{"1"}).Scan(&rows).Error
			if err != nil {
				m.log.Warn("[ DataSecurity ] Not find ids:")
				return
			}

			admin := header.GetAdminAuth(c)
			delRows := []model.SecurityDataRecycleLog{}
			for _, v := range rows {
				data, _ := json.Marshal(v)
				delRows = append(delRows, model.SecurityDataRecycleLog{
					AdminID:    admin.Id,
					RecycleID:  recycle.ID,
					Data:       string(data),
					DataTable:  recycle.DataTable,
					PrimaryKey: recycle.PrimaryKey,
					IP:         c.ClientIP(),
					Useragent:  c.Request.Header.Get("User-Agent"),
				})
			}

			if len(delRows) == 0 {
				m.log.Warn("[ DataSecurity ] Failed to recycle data:")
				return
			}
			err = m.sqlDB.Model(&model.SecurityDataRecycle{}).Create(&delRows).Error
			if err != nil {
				return
			}
			return
		}

		// 记录修改数据操作
		if c.Request.Method == http.MethodPost {
			return
		}
	}
}
