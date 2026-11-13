package area

import (
	"context"

	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	persistence "buildadmin-go/internal/pkg/persistence"

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

// List 按 province/city 逐级下钻返回省（level 1）、市（level 2）或区县
// （level 3）的 {value,label} 列表；空 province 表示取省级列表。
// province/city 由调用方从请求解析后传入，本方法不读传输层元数据。
func (s *AreaModel) List(ctx context.Context, province, city string) (any, error) {
	whereS := "pid=? and level=?"
	pid := "0"
	level := "1"
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
