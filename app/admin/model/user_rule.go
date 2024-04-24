package model

const TableNameUserRule = "ba_user_rule"

// UserRule 会员菜单权限规则表
type UserRule struct {
	ID           int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`                                                                           // ID
	Pid          int32  `gorm:"column:pid;not null;comment:上级菜单" json:"pid"`                                                                                            // 上级菜单
	Type         string `gorm:"column:type;not null;default:menu;comment:类型:route=路由,menu_dir=菜单目录,menu=菜单项,nav_user_menu=顶栏会员菜单下拉项,nav=顶栏菜单项,button=页面按钮" json:"type"` // 类型:route=路由,menu_dir=菜单目录,menu=菜单项,nav_user_menu=顶栏会员菜单下拉项,nav=顶栏菜单项,button=页面按钮
	Title        string `gorm:"column:title;not null;comment:标题" json:"title"`                                                                                          // 标题
	Name         string `gorm:"column:name;not null;comment:规则名称" json:"name"`                                                                                          // 规则名称
	Path         string `gorm:"column:path;not null;comment:路由路径" json:"path"`                                                                                          // 路由路径
	Icon         string `gorm:"column:icon;not null;comment:图标" json:"icon"`                                                                                            // 图标
	MenuType     string `gorm:"column:menu_type;not null;default:tab;comment:菜单类型:tab=选项卡,link=链接,iframe=Iframe" json:"menu_type"`                                      // 菜单类型:tab=选项卡,link=链接,iframe=Iframe
	URL          string `gorm:"column:url;not null;comment:Url" json:"url"`                                                                                             // Url
	Component    string `gorm:"column:component;not null;comment:组件路径" json:"component"`                                                                                // 组件路径
	NoLoginValid int32  `gorm:"column:no_login_valid;not null;comment:未登录有效:0=否,1=是" json:"no_login_valid"`                                                             // 未登录有效:0=否,1=是
	Extend       string `gorm:"column:extend;not null;default:none;comment:扩展属性:none=无,add_rules_only=只添加为路由,add_menu_only=只添加为菜单" json:"extend"`                       // 扩展属性:none=无,add_rules_only=只添加为路由,add_menu_only=只添加为菜单
	Remark       string `gorm:"column:remark;not null;comment:备注" json:"remark"`                                                                                        // 备注
	Weigh        int32  `gorm:"column:weigh;not null;comment:权重" json:"weigh"`                                                                                          // 权重
	Status       string `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"`                                                                    // 状态:0=禁用,1=启用
	UpdateTime   int64  `gorm:"column:update_time;comment:更新时间" json:"update_time"`                                                                                     // 更新时间
	CreateTime   int64  `gorm:"column:create_time;comment:创建时间" json:"create_time"`                                                                                     // 创建时间
}

// TableName UserRule's table name
func (*UserRule) TableName() string {
	return TableNameUserRule
}
