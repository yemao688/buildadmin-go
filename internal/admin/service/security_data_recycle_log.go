package service

import (
	"context"
	"encoding/json"
	"fmt"

	securitymodel "buildadmin-go/internal/admin/repository"

	"gorm.io/gorm"
)

// SecurityDataRecycleLogService 承载回收站日志恢复的业务编排：id 归一化、
// 事务内锁定待恢复日志、目标表 fail-closed 校验、历史规则主键一致性检查
// 与逐条恢复。repo 只保留锁定/解析/写入原子原语。
type SecurityDataRecycleLogService struct {
	dataRecycleLogM *securitymodel.SecurityDataRecycleLogRepository
}

func NewSecurityDataRecycleLogService(dataRecycleLogM *securitymodel.SecurityDataRecycleLogRepository) *SecurityDataRecycleLogService {
	return &SecurityDataRecycleLogService{dataRecycleLogM: dataRecycleLogM}
}

// Restore 恢复一批回收站日志：任何一条失败（无效目标、规则不可用、主键
// 不匹配、JSON 损坏）都会让整个批次原子回退。
func (s *SecurityDataRecycleLogService) Restore(ctx context.Context, ids []int32) error {
	normalized, err := normalizeLogIDs(ids, "recycle log")
	if err != nil {
		return err
	}
	return s.dataRecycleLogM.Transaction(ctx, func(tx *gorm.DB) error {
		list, err := s.dataRecycleLogM.LockPendingLogs(tx, normalized)
		if err != nil {
			return err
		}
		for _, v := range list {
			targetTable, err := s.dataRecycleLogM.ResolveTarget(tx, v.DataTable, v.PrimaryKey)
			if err != nil {
				return err
			}
			data := map[string]any{}
			if err := json.Unmarshal([]byte(v.Data), &data); err != nil {
				return err
			}

			// Fail-closed: refuse to restore into tables that cannot carry ownership.
			rule, err := s.dataRecycleLogM.RecycleRuleByIDTx(tx, v.RecycleID)
			if err != nil {
				return fmt.Errorf("recycle rule %d unavailable: %w", v.RecycleID, err)
			}
			if rule.PrimaryKey == "" {
				rule.PrimaryKey = "id"
			}
			if v.PrimaryKey != rule.PrimaryKey {
				return fmt.Errorf("recycle log primary key does not match historical rule")
			}
			if err := s.dataRecycleLogM.CreateRowTx(tx, targetTable, data); err != nil {
				return err
			}
			if err := s.dataRecycleLogM.MarkRestoredTx(tx, v.ID); err != nil {
				return err
			}
		}
		return nil
	})
}
