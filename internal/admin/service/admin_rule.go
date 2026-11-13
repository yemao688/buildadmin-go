package service

import (
	"context"
	"slices"
	"strconv"

	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"

	"gorm.io/gorm"
)

// AdminRuleService 承载后台权限规则（菜单）的业务编排：父子环修正、
// "先删子级"删除约束，以及按操作者权限范围读取菜单。
// repo 只保留原子写原语与 scoped 读。
type AdminRuleService struct {
	adminRuleM *adminmodel.AdminRuleRepository
	authM      *adminmodel.AuthRepository
}

func NewAdminRuleService(adminRuleM *adminmodel.AdminRuleRepository, authM *adminmodel.AuthRepository) *AdminRuleService {
	return &AdminRuleService{adminRuleM: adminRuleM, authM: authM}
}

// Menus 返回操作者可见的规则列表。超管（规则集合含 "*"）不按权限 id 过滤，
// 非超管只返回自己规则集合内的项；types/status 为空表示不限制，keyword 交由
// repo 做 QuickSearch 匹配。树装配属于响应层，仍留在 handler。
func (s *AdminRuleService) Menus(ctx context.Context, operatorID int32, keyword string, types []string, status string) ([]model.AdminRule, error) {
	ids, err := s.authM.GetRuleIds(operatorID)
	if err != nil {
		return nil, err
	}
	isSuperAdmin := slices.Contains(ids, "*")
	ruleIDs := make([]int32, 0, len(ids))
	for _, raw := range ids {
		if raw == "*" {
			continue
		}
		id, convErr := strconv.Atoi(raw)
		if convErr != nil {
			continue
		}
		ruleIDs = append(ruleIDs, int32(id))
	}
	return s.adminRuleM.ListForMenu(ctx, isSuperAdmin, ruleIDs, keyword, types, status)
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
