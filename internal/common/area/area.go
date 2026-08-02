package area

import (
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	persistence "buildadmin-go/internal/pkg/persistence"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Area 省份地区表
type Area = model.Area

type AreaModel struct {
	persistence.BaseModel
}

func NewAreaModel(sqlDB *gorm.DB, config *conf.Configuration) *AreaModel {
	return &AreaModel{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"area", "id", "name", sqlDB),
	}
}

func (s *AreaModel) List(ctx *gin.Context) (any, error) {
	whereS := "pid=? and level=?"
	pid := "0"
	level := "1"
	province := ctx.Request.FormValue("province")
	city := ctx.Request.FormValue("city")
	if province != "" {
		pid = province
		level = "2"
		if city != "" {
			pid = city
			level = "3"
		}
	}

	list := []struct {
		Value int32  `json:"value"`
		Label string `json:"label"`
	}{}
	err := s.DB().Model(&Area{}).Select("id as value,name as label").Where(whereS, pid, level).Scan(&list).Error
	return list, err
}
