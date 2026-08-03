package service

import (
	"context"
	"slices"
	"strconv"
	"strings"

	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"
)

// AdminGroupService 承载后台分组的业务规则：权限节点装配（HandleRules）、
// 分组操作授权（CheckAuth）与增改删编排。列表/树装配仍留在 handler
// （响应层）。
type AdminGroupService struct {
	adminGroupM *adminmodel.AdminGroupRepository
	adminRuleM  *adminmodel.AdminRuleRepository
	authM       *adminmodel.AuthRepository
}

func NewAdminGroupService(adminGroupM *adminmodel.AdminGroupRepository, adminRuleM *adminmodel.AdminRuleRepository, authM *adminmodel.AuthRepository) *AdminGroupService {
	return &AdminGroupService{adminGroupM: adminGroupM, adminRuleM: adminRuleM, authM: authM}
}

// HandleRules 权限节点入库前处理：全部规则视为超级管理员（*），并禁止
// 添加"拥有自己全部权限"的分组。
func (s *AdminGroupService) HandleRules(ctx context.Context, rules []int32, operatorID int32) (string, error) {
	if len(rules) > 0 {
		list, err := s.adminRuleM.List(ctx)
		if err != nil {
			return "", err
		}
		// 判断是否超级管理员
		super := true
		for _, r := range list {
			if !slices.Contains(rules, r.ID) {
				super = false
				break
			}
		}
		if super {
			return "*", nil
		}

		stringRules := []string{}
		for _, v := range rules {
			stringRules = append(stringRules, strconv.Itoa(int(v)))
		}
		// 禁止添加`拥有自己全部权限`的分组
		hasRules, err := s.authM.GetRuleIds(operatorID)
		if err != nil {
			return "", err
		}
		isAll := true
		for _, v := range hasRules {
			if !slices.Contains(stringRules, v) {
				isAll = false
			}
		}
		if isAll {
			return "", cErr.BadRequest("Role group has all your rights, please contact the upper administrator to add or do not need to add!")
		}
		return strings.Join(stringRules, ","), nil
	}
	return "", nil
}

// CheckAuth 校验操作者是否有权操作该分组（需拥有该分组的全部权限且有
// 额外权限）。
func (s *AdminGroupService) CheckAuth(operatorID int32, isSuperAdmin bool, groupID int32) error {
	if isSuperAdmin {
		return nil
	}
	authGroups, err := s.authM.GetAllAuthGroups("allAuthAndOthers", operatorID)
	if err != nil {
		return err
	}
	idStr := strconv.Itoa(int(groupID))
	if !slices.Contains(authGroups, idStr) {
		return cErr.BadRequest("You need to have all the permissions of the group and have additional permissions before you can operate the group~")
	}
	return nil
}

// Add 编排分组创建：规则装配 + 落库。
func (s *AdminGroupService) Add(ctx context.Context, group model.AdminGroup, ruleIDs []int32, operatorID int32) error {
	rules, err := s.HandleRules(ctx, ruleIDs, operatorID)
	if err != nil {
		return err
	}
	group.Rules = rules
	return s.adminGroupM.Add(ctx, group)
}

// Edit 编排分组更新：重载 + 授权 + 禁止修改自己的分组 + 规则装配 + 落库。
func (s *AdminGroupService) Edit(ctx context.Context, id int32, params model.AdminGroup, ruleIDs []int32, operatorID int32, isSuperAdmin bool) error {
	adminGroup, err := s.adminGroupM.GetOne(ctx, id)
	if err != nil {
		return err
	}
	if err := s.CheckAuth(operatorID, isSuperAdmin, id); err != nil {
		return err
	}

	groupIds := s.authM.GetGroupIds(operatorID)
	if slices.Contains(groupIds, id) {
		return cErr.BadRequest("You cannot modify your own management group!")
	}

	if err := copyGroup(&adminGroup, &params); err != nil {
		return err
	}
	adminGroup.Rules, err = s.HandleRules(ctx, ruleIDs, operatorID)
	if err != nil {
		return err
	}
	return s.adminGroupM.Edit(ctx, adminGroup)
}

// Del 编排分组删除：逐个授权 + 落库（含"不能删除自己所在分组"约束）。
func (s *AdminGroupService) Del(ctx context.Context, ids []int32, operatorID int32, isSuperAdmin bool) error {
	for _, v := range ids {
		if err := s.CheckAuth(operatorID, isSuperAdmin, v); err != nil {
			return err
		}
	}
	return s.adminGroupM.DelWithOperator(ctx, ids, operatorID)
}

func copyGroup(dst, src *model.AdminGroup) error {
	dst.Pid = src.Pid
	dst.Name = src.Name
	dst.Status = src.Status
	return nil
}
