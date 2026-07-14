package migrations

import (
	"errors"
	"go-build-admin/database/migrations/model"
	"time"

	"gorm.io/gorm"
)

type Install struct {
	sqlDB *gorm.DB
}

func NewInstall(sqlDB *gorm.DB) *Install {
	return &Install{
		sqlDB: sqlDB,
	}
}

func (s Install) InsertData() error {
	return s.sqlDB.Transaction(func(tx *gorm.DB) error {
		seed := Install{sqlDB: tx}
		for _, fn := range []func() error{seed.AdminGroupAccess, seed.AdminGroup, seed.AdminRule, seed.Admin, seed.Config, seed.SecurityDataRecycle, seed.SecuritySensitiveData, seed.UserGroup, seed.UserRule, seed.User} {
			if err := fn(); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s Install) AdminGroupAccess() error {
	err := s.sqlDB.Where("uid=?", "1").First(&model.AdminGroupAccess{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		dataList := []*model.AdminGroupAccess{
			{
				UID:     1,
				GroupID: 1,
			},
		}
		if err := s.sqlDB.Create(dataList).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func (s Install) AdminGroup() error {
	err := s.sqlDB.Where("id=?", "1").First(&model.AdminGroup{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		dataList := []*model.AdminGroup{
			{
				ID:         1,
				Pid:        0,
				Name:       "超级管理组",
				Rules:      "*",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         2,
				Pid:        1,
				Name:       "一级管理员",
				Rules:      "1,21,22,23,24,25,26,27,28,29,30,31,32,33,34,35,36,37,38,39,40,41,42,43,44,45,46,47,77,48,49,50,51,52,53,54,55,56,57,58,59,60,61,62,63,64,65,66,67,68,69,70,71,72,73,74,75,76,89",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         3,
				Pid:        2,
				Name:       "二级管理员",
				Rules:      "21,22,23,24,25,26,27,28,29,30,31,32,33,34,35,36,37,38,39,40,41,42,43",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         4,
				Pid:        3,
				Name:       "三级管理员",
				Rules:      "55,56,57,58,59,60,61,62,63,64,65,66,67,68,69,70,71,72,73,74,75",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
		}
		if err := s.sqlDB.Create(dataList).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s Install) AdminRule() error {
	err := s.sqlDB.Where("id=?", "1").First(&model.AdminRule{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		dataList := []*model.AdminRule{
			{
				ID:         1,
				Type:       "menu",
				Title:      "控制台",
				Name:       "dashboard",
				Path:       "dashboard",
				Icon:       "fa fa-dashboard",
				MenuType:   "tab",
				Component:  "/src/views/backend/dashboard.vue",
				Keepalive:  1,
				Remark:     "Remark lang",
				Weigh:      999,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         2,
				Type:       "menu_dir",
				Title:      "权限管理",
				Name:       "auth",
				Path:       "auth",
				Icon:       "fa fa-group",
				Weigh:      100,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         3,
				Pid:        2,
				Type:       "menu",
				Title:      "角色组管理",
				Name:       "auth/group",
				Path:       "auth/group",
				Icon:       "fa fa-group",
				MenuType:   "tab",
				Component:  "/src/views/backend/auth/group/index.vue",
				Keepalive:  1,
				Weigh:      99,
				Remark:     "Remark lang",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         4,
				Pid:        3,
				Type:       "button",
				Title:      "查看",
				Name:       "auth/group/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         5,
				Pid:        3,
				Type:       "button",
				Title:      "添加",
				Name:       "auth/group/add",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         6,
				Pid:        3,
				Type:       "button",
				Title:      "编辑",
				Name:       "auth/group/edit",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         7,
				Pid:        3,
				Type:       "button",
				Title:      "删除",
				Name:       "auth/group/del",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         8,
				Pid:        2,
				Type:       "menu",
				Title:      "管理员管理",
				Name:       "auth/admin",
				Path:       "auth/admin",
				Icon:       "el-icon-UserFilled",
				MenuType:   "tab",
				Component:  "/src/views/backend/auth/admin/index.vue",
				Keepalive:  1,
				Weigh:      98,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         9,
				Pid:        8,
				Type:       "button",
				Title:      "查看",
				Name:       "auth/admin/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         10,
				Pid:        8,
				Type:       "button",
				Title:      "添加",
				Name:       "auth/admin/add",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         11,
				Pid:        8,
				Type:       "button",
				Title:      "编辑",
				Name:       "auth/admin/edit",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         12,
				Pid:        8,
				Type:       "button",
				Title:      "删除",
				Name:       "auth/admin/del",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         13,
				Pid:        2,
				Type:       "menu",
				Title:      "菜单规则管理",
				Name:       "auth/rule",
				Path:       "auth/rule",
				Icon:       "el-icon-Grid",
				MenuType:   "tab",
				Component:  "/src/views/backend/auth/rule/index.vue",
				Keepalive:  1,
				Weigh:      97,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         14,
				Pid:        13,
				Type:       "button",
				Title:      "查看",
				Name:       "auth/rule/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         15,
				Pid:        13,
				Type:       "button",
				Title:      "添加",
				Name:       "auth/rule/add",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         16,
				Pid:        13,
				Type:       "button",
				Title:      "编辑",
				Name:       "auth/rule/edit",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         17,
				Pid:        13,
				Type:       "button",
				Title:      "删除",
				Name:       "auth/rule/del",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         18,
				Pid:        13,
				Type:       "button",
				Title:      "快速排序",
				Name:       "auth/rule/sortable",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         19,
				Pid:        2,
				Type:       "menu",
				Title:      "管理员日志管理",
				Name:       "auth/adminLog",
				Path:       "auth/adminLog",
				Icon:       "el-icon-List",
				MenuType:   "tab",
				Component:  "/src/views/backend/auth/adminLog/index.vue",
				Keepalive:  1,
				Weigh:      96,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         20,
				Pid:        19,
				Type:       "button",
				Title:      "查看",
				Name:       "auth/adminLog/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         21,
				Type:       "menu_dir",
				Title:      "会员管理",
				Name:       "user",
				Path:       "user",
				Icon:       "fa fa-drivers-license",
				Weigh:      95,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         22,
				Pid:        21,
				Type:       "menu",
				Title:      "会员管理",
				Name:       "user/user",
				Path:       "user/user",
				Icon:       "fa fa-user",
				MenuType:   "tab",
				Component:  "/src/views/backend/user/user/index.vue",
				Keepalive:  1,
				Weigh:      94,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         23,
				Pid:        22,
				Type:       "button",
				Title:      "查看",
				Name:       "user/user/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         24,
				Pid:        22,
				Type:       "button",
				Title:      "添加",
				Name:       "user/user/add",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         25,
				Pid:        22,
				Type:       "button",
				Title:      "编辑",
				Name:       "user/user/edit",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         26,
				Pid:        22,
				Type:       "button",
				Title:      "删除",
				Name:       "user/user/del",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         27,
				Pid:        21,
				Type:       "menu",
				Title:      "会员分组管理",
				Name:       "user/group",
				Path:       "user/group",
				Icon:       "fa fa-group",
				MenuType:   "tab",
				Component:  "/src/views/backend/user/group/index.vue",
				Keepalive:  1,
				Weigh:      93,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         28,
				Pid:        27,
				Type:       "button",
				Title:      "查看",
				Name:       "user/group/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         29,
				Pid:        27,
				Type:       "button",
				Title:      "添加",
				Name:       "user/group/add",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         30,
				Pid:        27,
				Type:       "button",
				Title:      "编辑",
				Name:       "user/group/edit",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         31,
				Pid:        27,
				Type:       "button",
				Title:      "删除",
				Name:       "user/group/del",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         32,
				Pid:        21,
				Type:       "menu",
				Title:      "会员规则管理",
				Name:       "user/rule",
				Path:       "user/rule",
				Icon:       "fa fa-th-list",
				MenuType:   "tab",
				Component:  "/src/views/backend/user/rule/index.vue",
				Keepalive:  1,
				Weigh:      92,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         33,
				Pid:        32,
				Type:       "button",
				Title:      "查看",
				Name:       "user/rule/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         34,
				Pid:        32,
				Type:       "button",
				Title:      "添加",
				Name:       "user/rule/add",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         35,
				Pid:        32,
				Type:       "button",
				Title:      "编辑",
				Name:       "user/rule/edit",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         36,
				Pid:        32,
				Type:       "button",
				Title:      "删除",
				Name:       "user/rule/del",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         37,
				Pid:        32,
				Type:       "button",
				Title:      "快速排序",
				Name:       "user/rule/sortable",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         38,
				Pid:        21,
				Type:       "menu",
				Title:      "会员余额管理",
				Name:       "user/moneyLog",
				Path:       "user/moneyLog",
				Icon:       "el-icon-Money",
				MenuType:   "tab",
				Component:  "/src/views/backend/user/moneyLog/index.vue",
				Keepalive:  1,
				Weigh:      91,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         39,
				Pid:        38,
				Type:       "button",
				Title:      "查看",
				Name:       "user/moneyLog/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         40,
				Pid:        38,
				Type:       "button",
				Title:      "添加",
				Name:       "user/moneyLog/add",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         41,
				Pid:        21,
				Type:       "menu",
				Title:      "会员积分管理",
				Name:       "user/scoreLog",
				Path:       "user/scoreLog",
				Icon:       "el-icon-Discount",
				MenuType:   "tab",
				Component:  "/src/views/backend/user/scoreLog/index.vue",
				Keepalive:  1,
				Weigh:      90,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         42,
				Pid:        41,
				Type:       "button",
				Title:      "查看",
				Name:       "user/scoreLog/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         43,
				Pid:        41,
				Type:       "button",
				Title:      "添加",
				Name:       "user/scoreLog/add",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         44,
				Type:       "menu_dir",
				Title:      "常规管理",
				Name:       "routine",
				Path:       "routine",
				Icon:       "fa fa-cogs",
				Weigh:      89,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         45,
				Pid:        44,
				Type:       "menu",
				Title:      "系统配置",
				Name:       "routine/config",
				Path:       "routine/config",
				Icon:       "el-icon-Tools",
				MenuType:   "tab",
				Component:  "/src/views/backend/routine/config/index.vue",
				Keepalive:  1,
				Weigh:      88,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         46,
				Pid:        45,
				Type:       "button",
				Title:      "查看",
				Name:       "routine/config/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         47,
				Pid:        45,
				Type:       "button",
				Title:      "编辑",
				Name:       "routine/config/edit",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         48,
				Pid:        44,
				Type:       "menu",
				Title:      "附件管理",
				Name:       "routine/attachment",
				Path:       "routine/attachment",
				Icon:       "fa fa-folder",
				MenuType:   "tab",
				Component:  "/src/views/backend/routine/attachment/index.vue",
				Keepalive:  1,
				Remark:     "Remark lang",
				Weigh:      87,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         49,
				Pid:        48,
				Type:       "button",
				Title:      "查看",
				Name:       "routine/attachment/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         50,
				Pid:        48,
				Type:       "button",
				Title:      "编辑",
				Name:       "routine/attachment/edit",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         51,
				Pid:        48,
				Type:       "button",
				Title:      "删除",
				Name:       "routine/attachment/del",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         52,
				Pid:        44,
				Type:       "menu",
				Title:      "个人资料",
				Name:       "routine/adminInfo",
				Path:       "routine/adminInfo",
				Icon:       "fa fa-user",
				MenuType:   "tab",
				Component:  "/src/views/backend/routine/adminInfo.vue",
				Keepalive:  1,
				Weigh:      86,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         53,
				Pid:        52,
				Type:       "button",
				Title:      "查看",
				Name:       "routine/adminInfo/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         54,
				Pid:        52,
				Type:       "button",
				Title:      "编辑",
				Name:       "routine/adminInfo/edit",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         55,
				Type:       "menu_dir",
				Title:      "数据安全管理",
				Name:       "security",
				Path:       "security",
				Icon:       "fa fa-shield",
				Weigh:      85,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         56,
				Pid:        55,
				Type:       "menu",
				Title:      "数据回收站",
				Name:       "security/dataRecycleLog",
				Path:       "security/dataRecycleLog",
				Icon:       "fa fa-database",
				MenuType:   "tab",
				Component:  "/src/views/backend/security/dataRecycleLog/index.vue",
				Keepalive:  1,
				Weigh:      84,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         57,
				Pid:        56,
				Type:       "button",
				Title:      "查看",
				Name:       "security/dataRecycleLog/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         58,
				Pid:        56,
				Type:       "button",
				Title:      "删除",
				Name:       "security/dataRecycleLog/del",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         59,
				Pid:        56,
				Type:       "button",
				Title:      "还原",
				Name:       "security/dataRecycleLog/restore",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         60,
				Pid:        56,
				Type:       "button",
				Title:      "查看详情",
				Name:       "security/dataRecycleLog/info",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         61,
				Pid:        55,
				Type:       "menu",
				Title:      "敏感数据修改记录",
				Name:       "security/sensitiveDataLog",
				Path:       "security/sensitiveDataLog",
				Icon:       "fa fa-expeditedssl",
				MenuType:   "tab",
				Component:  "/src/views/backend/security/sensitiveDataLog/index.vue",
				Keepalive:  1,
				Weigh:      83,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         62,
				Pid:        61,
				Type:       "button",
				Title:      "查看",
				Name:       "security/sensitiveDataLog/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         63,
				Pid:        61,
				Type:       "button",
				Title:      "删除",
				Name:       "security/sensitiveDataLog/del",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         64,
				Pid:        61,
				Type:       "button",
				Title:      "回滚",
				Name:       "security/sensitiveDataLog/rollback",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         65,
				Pid:        61,
				Type:       "button",
				Title:      "查看详情",
				Name:       "security/sensitiveDataLog/info",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         66,
				Pid:        55,
				Type:       "menu",
				Title:      "数据回收规则管理",
				Name:       "security/dataRecycle",
				Path:       "security/dataRecycle",
				Icon:       "fa fa-database",
				MenuType:   "tab",
				Component:  "/src/views/backend/security/dataRecycle/index.vue",
				Keepalive:  1,
				Remark:     "Remark lang",
				Weigh:      82,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         67,
				Pid:        66,
				Type:       "button",
				Title:      "查看",
				Name:       "security/dataRecycle/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         68,
				Pid:        66,
				Type:       "button",
				Title:      "添加",
				Name:       "security/dataRecycle/add",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         69,
				Pid:        66,
				Type:       "button",
				Title:      "编辑",
				Name:       "security/dataRecycle/edit",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         70,
				Pid:        66,
				Type:       "button",
				Title:      "删除",
				Name:       "security/dataRecycle/del",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         71,
				Pid:        55,
				Type:       "menu",
				Title:      "敏感字段规则管理",
				Name:       "security/sensitiveData",
				Path:       "security/sensitiveData",
				Icon:       "fa fa-expeditedssl",
				MenuType:   "tab",
				Component:  "/src/views/backend/security/sensitiveData/index.vue",
				Keepalive:  1,
				Remark:     "Remark lang",
				Weigh:      81,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         72,
				Pid:        71,
				Type:       "button",
				Title:      "查看",
				Name:       "security/sensitiveData/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         73,
				Pid:        71,
				Type:       "button",
				Title:      "添加",
				Name:       "security/sensitiveData/add",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         74,
				Pid:        71,
				Type:       "button",
				Title:      "编辑",
				Name:       "security/sensitiveData/edit",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         75,
				Pid:        71,
				Type:       "button",
				Title:      "删除",
				Name:       "security/sensitiveData/del",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         76,
				Type:       "menu",
				Title:      "BuildAdmin",
				Name:       "buildadmin",
				Path:       "buildadmin",
				Icon:       "local-logo",
				MenuType:   "link",
				URL:        "https://doc.buildadmin.com",
				Status:     "0",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         77,
				Pid:        45,
				Type:       "button",
				Title:      "添加",
				Name:       "routine/config/add",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         78,
				Type:       "menu",
				Title:      "模块市场",
				Name:       "moduleStore/moduleStore",
				Path:       "moduleStore",
				Icon:       "el-icon-GoodsFilled",
				MenuType:   "tab",
				Component:  "/src/views/backend/module/index.vue",
				Keepalive:  1,
				Weigh:      86,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         79,
				Pid:        78,
				Type:       "button",
				Title:      "查看",
				Name:       "moduleStore/moduleStore/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         80,
				Pid:        78,
				Type:       "button",
				Title:      "安装",
				Name:       "moduleStore/moduleStore/install",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         81,
				Pid:        78,
				Type:       "button",
				Title:      "调整状态",
				Name:       "moduleStore/moduleStore/changeState",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         82,
				Pid:        78,
				Type:       "button",
				Title:      "卸载",
				Name:       "moduleStore/moduleStore/uninstall",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         83,
				Pid:        78,
				Type:       "button",
				Title:      "更新",
				Name:       "moduleStore/moduleStore/update",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         84,
				Type:       "menu",
				Title:      "CRUD代码生成",
				Name:       "crud/crud",
				Path:       "crud/crud",
				Icon:       "fa fa-code",
				MenuType:   "tab",
				Component:  "/src/views/backend/crud/index.vue",
				Keepalive:  1,
				Weigh:      80,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         85,
				Pid:        84,
				Type:       "button",
				Title:      "查看",
				Name:       "crud/crud/index",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         86,
				Pid:        84,
				Type:       "button",
				Title:      "生成",
				Name:       "crud/crud/generate",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         87,
				Pid:        84,
				Type:       "button",
				Title:      "删除",
				Name:       "crud/crud/delete",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         88,
				Pid:        45,
				Type:       "button",
				Title:      "删除",
				Name:       "routine/config/del",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         89,
				Pid:        1,
				Type:       "button",
				Title:      "查看",
				Name:       "dashboard/index",
				Status:     "1",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
		}
		if err := s.sqlDB.Create(dataList).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func (s Install) Admin() error {
	err := s.sqlDB.Where("id=?", "1").First(&model.Admin{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		dataList := []*model.Admin{
			{
				ID:         1,
				Username:   "admin",
				Nickname:   "Admin",
				Avatar:     "",
				Email:      "admin@buildadmin.com",
				Mobile:     "18888888888",
				Status:     "enable",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
		}
		if err := s.sqlDB.Create(dataList).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func (s Install) Config() error {
	err := s.sqlDB.Where("id=?", "1").First(&model.Config{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		dataList := []*model.Config{
			{
				ID:      1,
				Name:    "config_group",
				Group:   "basics",
				Title:   "Config group",
				Type:    "array",
				Value:   `[{"key":"basics","value":"Basics"},{"key":"mail","value":"Mail"},{"key":"config_quick_entrance","value":"Config Quick entrance"}]`,
				Content: "",
				Rule:    "required",
				Weigh:   -1,
			},
			{
				ID:      2,
				Name:    "site_name",
				Group:   "basics",
				Title:   "Site Name",
				Tip:     "站点名称",
				Type:    "string",
				Value:   "站点名称",
				Content: "",
				Rule:    "required",
				Weigh:   99,
			},
			{
				ID:    3,
				Name:  "record_number",
				Group: "basics",
				Title: "Record number",
				Tip:   "域名备案号",
				Type:  "string",
				Value: "渝ICP备8888888号-1",
			},
			{
				ID:    4,
				Name:  "version",
				Group: "basics",
				Title: "Version number",
				Tip:   "系统版本号",
				Type:  "string",
				Value: "v1.0.0",
				Rule:  "required",
			},
			{
				ID:    5,
				Name:  "time_zone",
				Group: "basics",
				Title: "time zone",
				Type:  "string",
				Value: "Asia/Shanghai",
				Rule:  "required",
			},
			{
				ID:    6,
				Name:  "no_access_ip",
				Group: "basics",
				Title: "No access ip",
				Tip:   "禁止访问站点的ip列表,一行一个",
				Type:  "textarea",
			},
			{
				ID:    7,
				Name:  "smtp_server",
				Group: "mail",
				Title: "smtp server",
				Type:  "string",
				Value: "smtp.qq.com",
				Weigh: 9,
			},
			{
				ID:    8,
				Name:  "smtp_port",
				Group: "mail",
				Title: "smtp port",
				Type:  "string",
				Value: "465",
				Weigh: 8,
			},
			{
				ID:    9,
				Name:  "smtp_user",
				Group: "mail",
				Title: "smtp user",
				Type:  "string",
				Weigh: 7,
			},
			{
				ID:    10,
				Name:  "smtp_pass",
				Group: "mail",
				Title: "smtp pass",
				Type:  "string",
				Weigh: 6,
			},
			{
				ID:      11,
				Name:    "smtp_verification",
				Group:   "mail",
				Title:   "smtp verification",
				Type:    "select",
				Value:   "SSL",
				Content: `{"SSL":"SSL","TLS":"TLS"}`,
				Weigh:   5,
			},
			{
				ID:    12,
				Name:  "smtp_sender_mail",
				Group: "mail",
				Title: "smtp sender mail",
				Type:  "string",
				Rule:  "email",
				Weigh: 4,
			},
			{
				ID:    13,
				Name:  "config_quick_entrance",
				Group: "config_quick_entrance",
				Title: "Config Quick entrance",
				Type:  "array",
				Value: `[{"key":"数据回收规则配置","value":"security/dataRecycle"},{"key":"敏感数据规则配置","value":"security/sensitiveData"}]`,
			},
		}
		if err := s.sqlDB.Create(dataList).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func (s Install) SecurityDataRecycle() error {
	err := s.sqlDB.Where("id=?", "1").First(&model.SecurityDataRecycle{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		dataList := []*model.SecurityDataRecycle{
			{
				ID:           1,
				Name:         "管理员",
				Controller:   "auth/Admin.php",
				ControllerAs: "auth/admin",
				DataTable:    "admin",
				PrimaryKey:   "id",
				UpdateTime:   time.Now().Unix(),
				CreateTime:   time.Now().Unix(),
			},
			{
				ID:           2,
				Name:         "管理员日志",
				Controller:   "auth/AdminLog.php",
				ControllerAs: "auth/adminlog",
				DataTable:    "admin_log",
				PrimaryKey:   "id",
				UpdateTime:   time.Now().Unix(),
				CreateTime:   time.Now().Unix(),
			},
			{
				ID:           3,
				Name:         "菜单规则",
				Controller:   "auth/Menu.php",
				ControllerAs: "auth/rule",
				DataTable:    "admin_rule",
				PrimaryKey:   "id",
				UpdateTime:   time.Now().Unix(),
				CreateTime:   time.Now().Unix(),
			},
			{
				ID:           4,
				Name:         "系统配置项",
				Controller:   "routine/Config.php",
				ControllerAs: "routine/config",
				DataTable:    "config",
				PrimaryKey:   "id",
				UpdateTime:   time.Now().Unix(),
				CreateTime:   time.Now().Unix(),
			},
			{
				ID:           5,
				Name:         "会员",
				Controller:   "user/User.php",
				ControllerAs: "user/user",
				DataTable:    "user",
				PrimaryKey:   "id",
				UpdateTime:   time.Now().Unix(),
				CreateTime:   time.Now().Unix(),
			},
			{
				ID:           6,
				Name:         "数据回收规则",
				Controller:   "security/DataRecycle.php",
				ControllerAs: "security/datarecycle",
				DataTable:    "security_data_recycle",
				PrimaryKey:   "id",
				UpdateTime:   time.Now().Unix(),
				CreateTime:   time.Now().Unix(),
			},
		}
		if err := s.sqlDB.Create(dataList).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func (s Install) SecuritySensitiveData() error {
	err := s.sqlDB.Where("id=?", "1").First(&model.SecuritySensitiveData{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {

		dataList := []*model.SecuritySensitiveData{
			{
				ID:           1,
				Name:         "管理员数据",
				Controller:   "auth/Admin.php",
				ControllerAs: "auth/admin",
				DataTable:    "admin",
				PrimaryKey:   "id",
				DataFields:   `{"username":"用户名","mobile":"手机","password":"密码","status":"状态"}`,
				Status:       "1",
				UpdateTime:   time.Now().Unix(),
				CreateTime:   time.Now().Unix(),
			},
			{
				ID:           2,
				Name:         "会员数据",
				Controller:   "user/User.php",
				ControllerAs: "user/user",
				DataTable:    "user",
				PrimaryKey:   "id",
				DataFields:   `{"username":"用户名","mobile":"手机号","password":"密码","status":"状态","email":"邮箱地址"}`,
				Status:       "1",
				UpdateTime:   time.Now().Unix(),
				CreateTime:   time.Now().Unix(),
			},
			{
				ID:           3,
				Name:         "管理员权限",
				Controller:   "auth/Group.php",
				ControllerAs: "auth/group",
				DataTable:    "admin_group",
				PrimaryKey:   "id",
				DataFields:   `{"rules":"权限规则ID"}`,
				Status:       "1",
				UpdateTime:   time.Now().Unix(),
				CreateTime:   time.Now().Unix(),
			},
		}
		if err := s.sqlDB.Create(dataList).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func (s Install) UserGroup() error {
	err := s.sqlDB.Where("id=?", "1").First(&model.UserGroup{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {

		dataList := []*model.UserGroup{
			{
				ID:         1,
				Name:       "默认分组",
				Rules:      "*",
				Status:     "1",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
		}
		if err := s.sqlDB.Create(dataList).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func (s Install) UserRule() error {
	err := s.sqlDB.Where("id=?", "1").First(&model.UserRule{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		dataList := []*model.UserRule{
			{
				ID:         1,
				Pid:        0,
				Type:       "menu_dir",
				Title:      "我的账户",
				Name:       "account",
				Path:       "account",
				Icon:       "fa fa-user-circle",
				MenuType:   "tab",
				Weigh:      98,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         2,
				Pid:        1,
				Type:       "menu",
				Title:      "账户概览",
				Name:       "account/overview",
				Path:       "account/overview",
				Icon:       "fa fa-home",
				MenuType:   "tab",
				Component:  "/src/views/frontend/user/account/overview.vue",
				Weigh:      99,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         3,
				Pid:        1,
				Type:       "menu",
				Title:      "个人资料",
				Name:       "account/profile",
				Path:       "account/profile",
				Icon:       "fa fa-user-circle-o",
				MenuType:   "tab",
				Component:  "/src/views/frontend/user/account/profile.vue",
				Weigh:      98,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         4,
				Pid:        1,
				Type:       "menu",
				Title:      "修改密码",
				Name:       "account/changePassword",
				Path:       "account/changePassword",
				Icon:       "fa fa-shield",
				MenuType:   "tab",
				Component:  "/src/views/frontend/user/account/changePassword.vue",
				Weigh:      97,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         5,
				Pid:        1,
				Type:       "menu",
				Title:      "积分记录",
				Name:       "account/integral",
				Path:       "account/integral",
				Icon:       "fa fa-tag",
				MenuType:   "tab",
				Component:  "/src/views/frontend/user/account/integral.vue",
				Weigh:      96,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
			{
				ID:         6,
				Pid:        1,
				Type:       "menu",
				Title:      "余额记录",
				Name:       "account/balance",
				Path:       "account/balance",
				Icon:       "fa fa-money",
				MenuType:   "tab",
				Component:  "/src/views/frontend/user/account/balance.vue",
				Weigh:      95,
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
		}
		if err := s.sqlDB.Create(dataList).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func (s Install) User() error {
	err := s.sqlDB.Where("id=?", "1").First(&model.User{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {

		dataList := []*model.User{
			{
				ID:         1,
				GroupID:    1,
				Username:   "user",
				Nickname:   "User",
				Email:      "18888888888@qq.com",
				Mobile:     "18888888888",
				Gender:     2,
				Birthday:   time.Now(),
				Status:     "enable",
				UpdateTime: time.Now().Unix(),
				CreateTime: time.Now().Unix(),
			},
		}
		if err := s.sqlDB.Create(dataList).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}
