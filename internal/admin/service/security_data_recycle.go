package service

import (
	"context"
	"fmt"

	securitymodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"

	"gorm.io/gorm"
)

// SecurityDataRecycleService 承载回收站规则（security_data_recycle）的业务
// 编排：主键默认值、目标表策略校验、主键一致性检查与 controller_as 唯一性
// 约束。repo 只保留 scoped 原子原语与策略解析助手。
type SecurityDataRecycleService struct {
	dataRecycleM *securitymodel.SecurityDataRecycleRepository
}

func NewSecurityDataRecycleService(dataRecycleM *securitymodel.SecurityDataRecycleRepository) *SecurityDataRecycleService {
	return &SecurityDataRecycleService{dataRecycleM: dataRecycleM}
}

// Add 编排回收站规则新增：主键默认值 + 事务内策略校验与唯一性约束。
func (s *SecurityDataRecycleService) Add(ctx context.Context, data model.SecurityDataRecycle) error {
	if data.PrimaryKey == "" {
		data.PrimaryKey = "id"
	}
	return s.dataRecycleM.Transaction(ctx, func(tx *gorm.DB) error {
		policy, err := s.dataRecycleM.ResolvePolicyTx(tx, data.DataTable, "recycle", data.PrimaryKey, nil)
		if err != nil {
			return err
		}
		if policy.Table.PrimaryKey != data.PrimaryKey {
			return fmt.Errorf("invalid recycle rule primary key")
		}
		if data.Status == "1" {
			n, err := s.dataRecycleM.EnabledControllerAsCount(tx, data.ControllerAs, 0)
			if err != nil {
				return err
			}
			if n > 0 {
				return cErr.BadRequest("controller_as already has an enabled security rule")
			}
		}
		return s.dataRecycleM.CreateTx(tx, &data)
	})
}

// Edit 编排回收站规则更新：主键默认值 + 事务内策略校验与唯一性约束。
func (s *SecurityDataRecycleService) Edit(ctx context.Context, data model.SecurityDataRecycle) error {
	if data.PrimaryKey == "" {
		data.PrimaryKey = "id"
	}
	updates := map[string]any{
		"name": data.Name, "controller": data.Controller, "controller_as": data.ControllerAs,
		"data_table": data.DataTable, "primary_key": data.PrimaryKey, "status": data.Status,
		"connection": data.Connection,
	}
	return s.dataRecycleM.Transaction(ctx, func(tx *gorm.DB) error {
		if _, err := s.dataRecycleM.ResolvePolicyTx(tx, data.DataTable, "recycle", data.PrimaryKey, nil); err != nil {
			return err
		}
		if data.Status == "1" {
			n, err := s.dataRecycleM.EnabledControllerAsCount(tx, data.ControllerAs, data.ID)
			if err != nil {
				return err
			}
			if n > 0 {
				return cErr.BadRequest("controller_as already has an enabled security rule")
			}
		}
		return s.dataRecycleM.UpdateTx(tx, data.ID, updates)
	})
}

// UpdateStatus 编排状态开关：启用时先校验 controller_as 唯一性，再原子
// 更新状态。
func (s *SecurityDataRecycleService) UpdateStatus(ctx context.Context, id int32, status string) error {
	return s.dataRecycleM.Transaction(ctx, func(tx *gorm.DB) error {
		if status == "1" {
			current, err := s.dataRecycleM.GetByIDTx(tx, id)
			if err != nil {
				return err
			}
			n, err := s.dataRecycleM.EnabledControllerAsCount(tx, current.ControllerAs, id)
			if err != nil {
				return err
			}
			if n > 0 {
				return cErr.BadRequest("controller_as already has an enabled security rule")
			}
		}
		return s.dataRecycleM.UpdateStatusTx(tx, id, status)
	})
}
