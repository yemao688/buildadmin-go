package service

import (
	"context"
	"slices"

	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"

	"gorm.io/gorm"
)

// AdminRuleService 承载后台权限规则（菜单）的业务编排：父子环修正与
// "先删子级"删除约束。repo 只保留原子写原语与读取。
type AdminRuleService struct {
	adminRuleM *adminmodel.AdminRuleRepository
}

func NewAdminRuleService(adminRuleM *adminmodel.AdminRuleRepository) *AdminRuleService {
	return &AdminRuleService{adminRuleM: adminRuleM}
}

// Add 编排规则新增：事务内原子写入。
func (s *AdminRuleService) Add(ctx context.Context, adminRule model.AdminRule) error {
	return s.adminRuleM.Transaction(ctx, func(tx *gorm.DB) error {
		return s.adminRuleM.CreateTx(tx, adminRule)
	})
}

// Edit 编排规则更新：先把"以被编辑节点为父"的节点从环中摘除（父级循环
// 修正），再原子保存。
func (s *AdminRuleService) Edit(ctx context.Context, adminRule model.AdminRule) error {
	return s.adminRuleM.Transaction(ctx, func(tx *gorm.DB) error {
		var parent model.AdminRule
		if adminRule.Pid > 0 {
			p, err := s.adminRuleM.GetByIDTx(tx, adminRule.Pid)
			if err != nil {
				return err
			}
			parent = p
		}
		if parent.Pid == adminRule.ID {
			if err := s.adminRuleM.DetachParentTx(tx, parent.ID); err != nil {
				return err
			}
		}
		return s.adminRuleM.UpdateTx(tx, adminRule)
	})
}

// Del 编排规则删除：事务内"先删子级"约束 + 原子批量删除。
func (s *AdminRuleService) Del(ctx context.Context, ids []int32) error {
	return s.adminRuleM.Transaction(ctx, func(tx *gorm.DB) error {
		subIds, err := s.adminRuleM.ChildRuleIDs(tx, ids)
		if err != nil {
			return err
		}
		for _, v := range subIds {
			if !slices.Contains(ids, v) {
				return cErr.BadRequest("Please delete the child element first, or use batch deletion")
			}
		}
		return s.adminRuleM.DeleteTx(tx, ids)
	})
}
