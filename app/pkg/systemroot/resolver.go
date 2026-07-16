package systemroot

import (
	"fmt"

	"gorm.io/gorm"
)

// Resolver finds the configured system owner without assuming that its id is
// one. Runtime callers use it when assigning ownership to newly-created data.
type Resolver struct {
	DB         *gorm.DB
	AdminTable string
}

func (r Resolver) Resolve() (int32, error) {
	if r.DB == nil || r.AdminTable == "" {
		return 0, fmt.Errorf("system root administrator is unavailable")
	}
	var id int32
	if err := r.DB.Table(r.AdminTable).Where("id = 1").Pluck("id", &id).Error; err == nil && id > 0 {
		return id, nil
	}
	if err := r.DB.Table(r.AdminTable).Order("id ASC").Limit(1).Pluck("id", &id).Error; err != nil || id <= 0 {
		if err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("system root administrator is unavailable")
	}
	return id, nil
}
