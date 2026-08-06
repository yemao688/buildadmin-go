package service

import (
	"context"
	"fmt"

	securitymodel "buildadmin-go/internal/admin/repository"

	"gorm.io/gorm"
)

// SecuritySensitiveDataLogService 承载敏感数据日志回滚的业务编排：id 归一化、
// 事务内锁定待回滚日志、目标表/列 fail-closed 校验、历史规则主键一致性
// 检查与逐条回滚。repo 只保留锁定/解析/写入原子原语。
type SecuritySensitiveDataLogService struct {
	sensitiveDataLogM *securitymodel.SecuritySensitiveDataLogRepository
}

func NewSecuritySensitiveDataLogService(sensitiveDataLogM *securitymodel.SecuritySensitiveDataLogRepository) *SecuritySensitiveDataLogService {
	return &SecuritySensitiveDataLogService{sensitiveDataLogM: sensitiveDataLogM}
}

// Rollback 回滚一批敏感数据日志：任何一条失败（无效目标、规则不可用、
// 主键不匹配、行被并发修改）都会让整个批次原子回退。
func (s *SecuritySensitiveDataLogService) Rollback(ctx context.Context, ids []int32) error {
	normalized, err := normalizeIDs(ids, "sensitive data log", true, func(id int32) error {
		return fmt.Errorf("invalid sensitive data log id %d", id)
	})
	if err != nil {
		return err
	}
	return s.sensitiveDataLogM.Transaction(ctx, func(tx *gorm.DB) error {
		list, err := s.sensitiveDataLogM.LockPendingLogs(tx, normalized)
		if err != nil {
			return err
		}
		for _, v := range list {
			targetTable, err := s.sensitiveDataLogM.ResolveTarget(tx, v.DataTable, v.PrimaryKey, v.DataField)
			if err != nil {
				return err
			}
			// Fail-closed: refuse to rollback tables that cannot prove row ownership.
			rule, err := s.sensitiveDataLogM.SensitiveRuleByIDTx(tx, v.SensitiveID)
			if err != nil {
				return fmt.Errorf("sensitive rule %d unavailable: %w", v.SensitiveID, err)
			}
			if rule.PrimaryKey == "" {
				rule.PrimaryKey = "id"
			}
			if v.PrimaryKey != rule.PrimaryKey {
				return fmt.Errorf("sensitive log primary key does not match historical rule")
			}
			if err := s.sensitiveDataLogM.ApplyFieldRestoreTx(tx, targetTable, v.PrimaryKey, v.IDValue, v.DataField, v.Before, v.After); err != nil {
				return err
			}
			if err := s.sensitiveDataLogM.MarkRolledBackTx(tx, v.ID); err != nil {
				return err
			}
		}
		return nil
	})
}
