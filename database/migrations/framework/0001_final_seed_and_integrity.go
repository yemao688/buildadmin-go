package framework

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"go-build-admin/database/migrations/model"

	"gorm.io/gorm"
)

const userMoneyDecimalType = "decimal(12,2)"

var userMoneyDecimalColumns = []struct {
	table  string
	column string
}{
	{table: "user", column: "money"},
	{table: "user_money_log", column: "money"},
	{table: "user_money_log", column: "before"},
	{table: "user_money_log", column: "after"},
}

func finalSeedAndIntegrity(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	if err := EnsureAdminClosureSelfRows(db, config); err != nil {
		return err
	}
	if err := normalizeFreshSensitiveSeed(db, config); err != nil {
		return err
	}
	if err := seedCountryMenus(db, config); err != nil {
		return err
	}
	if err := seedCountryLanguages(db, config); err != nil {
		return err
	}
	return seedUploadConfig(db, config)
}

func normalizeFreshSensitiveSeed(db *gorm.DB, config *conf.Configuration) error {
	table := core.TableName(config, "security_sensitive_data")
	if err := db.Table(table).Where("id = ? AND name = ? AND controller = ? AND controller_as = ? AND data_table = ? AND primary_key = ?", 2, "会员数据", "user.User", "user/user", "user", "id").Update("data_fields", `{"username":"用户名","mobile":"手机号","status":"状态","email":"邮箱地址"}`).Error; err != nil {
		return fmt.Errorf("normalize %s sensitive seed: %w", table, err)
	}
	return nil
}

func seedCountryMenus(db *gorm.DB, config *conf.Configuration) error {
	table := core.QuoteIdentifier(core.TableName(config, "admin_rule"))
	ruleID := func(name string) (int32, error) {
		var id int32
		if err := db.Raw("SELECT id FROM "+table+" WHERE name = ?", name).Scan(&id).Error; err != nil {
			return 0, err
		}
		if id == 0 {
			return 0, fmt.Errorf("country menu %s was not seeded", name)
		}
		return id, nil
	}
	ensure := func(pid int32, ruleType, title, name, path, menuType, component string, weigh int) error {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM "+table+" WHERE name = ?", name).Scan(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return nil
		}
		return db.Exec("INSERT INTO "+table+" (pid, type, title, name, path, menu_type, component, weigh, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, '1')", pid, ruleType, title, name, path, menuType, component, weigh).Error
	}

	if err := ensure(0, "menu_dir", "国家管理", "country", "country", "", "", 0); err != nil {
		return fmt.Errorf("seed country menu dir: %w", err)
	}
	countryID, err := ruleID("country")
	if err != nil {
		return err
	}
	menus := []struct {
		title, name string
		weigh       int
	}{
		{"货币管理", "country/currency", 3},
		{"语言管理", "country/language", 2},
		{"语言文本管理", "country/languageContent", 1},
	}
	buttons := []struct{ title, suffix string }{
		{"查看", "/index"}, {"添加", "/add"}, {"编辑", "/edit"}, {"删除", "/del"}, {"快速排序", "/sortable"},
	}
	for _, menu := range menus {
		if err := ensure(countryID, "menu", menu.title, menu.name, menu.name, "tab", "/src/views/backend/"+menu.name+"/index.vue", menu.weigh); err != nil {
			return fmt.Errorf("seed country menu %s: %w", menu.name, err)
		}
		menuID, err := ruleID(menu.name)
		if err != nil {
			return err
		}
		for _, button := range buttons {
			if err := ensure(menuID, "button", button.title, menu.name+button.suffix, "", "", "", 0); err != nil {
				return fmt.Errorf("seed country menu %s%s: %w", menu.name, button.suffix, err)
			}
		}
	}
	return nil
}

func seedCountryLanguages(db *gorm.DB, config *conf.Configuration) error {
	table := core.TableName(config, "country_language")
	rows := []model.CountryLanguage{
		{Lan: "zh-cn", Name: "简体中文", Remark: "简体中文", Status: 1, Weigh: 2},
		{Lan: "en", Name: "English", Remark: "English", Status: 1, Weigh: 1},
	}
	for _, row := range rows {
		var count int64
		if err := db.Table(table).Where("lan = ?", row.Lan).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			continue
		}
		if err := db.Table(table).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedUploadConfig(db *gorm.DB, config *conf.Configuration) error {
	table := core.TableName(config, "config")
	var group model.Config
	err := db.Table(table).Where("name = ?", "config_group").Take(&group).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err == nil {
		changed, value, err := appendUploadConfigGroup(group.Value)
		if err != nil {
			return err
		}
		if changed {
			if err := db.Table(table).Where("id = ?", group.ID).Update("value", value).Error; err != nil {
				return err
			}
		}
	}
	for _, row := range aliossConfigRows() {
		var count int64
		if err := db.Table(table).Where("name = ?", row.Name).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			if err := db.Table(table).Create(&row).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func appendUploadConfigGroup(value string) (bool, string, error) {
	var items []map[string]string
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return false, value, nil
	}
	for _, item := range items {
		if item["key"] == "upload" {
			return false, value, nil
		}
	}
	items = append(items, map[string]string{"key": "upload", "value": "Upload"})
	raw, err := json.Marshal(items)
	if err != nil {
		return false, value, err
	}
	return true, string(raw), nil
}

func aliossConfigRows() []model.Config {
	return []model.Config{
		{ID: 14, Name: "upload_mode", Group: "upload", Title: "存储方式", Type: "select", Value: "framework", Content: `{"framework":"本地磁盘存储","alioss":"阿里云对象存储OSS"}`, Rule: "required", Weigh: 99},
		{ID: 15, Name: "upload_bucket", Group: "upload", Title: "Bucket名称", Tip: "请在阿里云对象存储控制台查询", Type: "string", Value: "", Rule: "", Weigh: 98},
		{ID: 16, Name: "upload_access_id", Group: "upload", Title: "AccessKey ID", Tip: "请在阿里云个人中心查询", Type: "string", Value: "", Rule: "", Weigh: 97},
		{ID: 17, Name: "upload_secret_key", Group: "upload", Title: "AccessKey Secret", Tip: "请在阿里云个人中心查询", Type: "string", Value: "", Rule: "", Weigh: 96},
		{ID: 18, Name: "upload_url", Group: "upload", Title: "存储区域", Tip: "请选择存储区域", Type: "select", Value: "", Content: `{"oss-cn-hangzhou":"华东1（杭州） oss-cn-hangzhou","oss-cn-shanghai":"华东2（上海） oss-cn-shanghai","oss-cn-nanjing":"华东5（南京本地地域） oss-cn-nanjing","oss-cn-fuzhou":"华东6（福州本地地域） oss-cn-fuzhou","oss-cn-qingdao":"华北1（青岛） oss-cn-qingdao","oss-cn-beijing":"华北2（北京） oss-cn-beijing","oss-cn-zhangjiakou":"华北 3（张家口） oss-cn-zhangjiakou","oss-cn-huhehaote":"华北5（呼和浩特） oss-cn-huhehaote","oss-cn-wulanchabu":"华北6（乌兰察布） oss-cn-wulanchabu","oss-cn-shenzhen":"华南1（深圳） oss-cn-shenzhen","oss-cn-heyuan":"华南2（河源） oss-cn-heyuan","oss-cn-guangzhou":"华南3（广州） oss-cn-guangzhou","oss-cn-chengdu":"西南1（成都） oss-cn-chengdu","oss-cn-hongkong":"中国（香港） oss-cn-hongkong","oss-us-west-1":"美国（硅谷） oss-us-west-1","oss-us-east-1":"美国（弗吉尼亚） oss-us-east-1","oss-ap-northeast-1":"日本（东京） oss-ap-northeast-1","oss-ap-northeast-2":"韩国（首尔） oss-ap-northeast-2","oss-ap-southeast-1":"新加坡 oss-ap-southeast-1","oss-ap-southeast-2":"澳大利亚（悉尼） oss-ap-southeast-2","oss-ap-southeast-3":"马来西亚（吉隆坡） oss-ap-southeast-3","oss-ap-southeast-5":"印度尼西亚（雅加达） oss-ap-southeast-5","oss-ap-southeast-6":"菲律宾（马尼拉） oss-ap-southeast-6","oss-ap-southeast-7":"泰国（曼谷） oss-ap-southeast-7","oss-ap-south-1":"印度（孟买） oss-ap-south-1","oss-eu-central-1":"德国（法兰克福） oss-eu-central-1","oss-eu-west-1":"英国（伦敦） oss-eu-west-1","oss-me-east-1":"阿联酋（迪拜） oss-me-east-1","oss-cn-hzjbp":"华东1金融云 oss-cn-hzjbp","oss-cn-shanghai-finance-1":"华东2金融云 oss-cn-shanghai-finance-1","oss-cn-beijing-finance-1":"华北2金融云 oss-cn-beijing-finance-1","oss-cn-shenzhen-finance-1":"华南1金融云 oss-cn-shenzhen-finance-1","oss-cn-hzfinance":"杭州金融云公网 oss-cn-hzfinance","oss-cn-shanghai-finance-1-pub":"上海金融云公网 oss-cn-shanghai-finance-1-pub","oss-cn-szfinance":"深圳金融云公网 oss-cn-szfinance","oss-cn-beijing-finance-1-pub":"北京金融云公网 oss-cn-beijing-finance-1-pub"}`, Rule: "", Weigh: 95},
		{ID: 19, Name: "upload_cdn_url", Group: "upload", Title: "CDN地址", Tip: "请输入阿里云对象存储的CDN加速域名，以http(s)://开头，比如：https://example.com", Type: "string", Value: "", Rule: "", Weigh: 94},
	}
}

func verifyFinalTableContractImpl(db *gorm.DB, config *conf.Configuration) error {
	for _, logical := range core.CoreLogicalNames() {
		if err := requireTable(db, core.TableName(config, logical)); err != nil {
			return err
		}
	}
	if err := verifyUserTableContract(db, config); err != nil {
		return err
	}
	if err := verifyAdminTableContract(db, config); err != nil {
		return err
	}
	if err := verifyStatusContract(db, config); err != nil {
		return err
	}
	if err := verifyMoneyDecimalContract(db, config); err != nil {
		return err
	}
	for _, logical := range []string{"user", "user_money_log", "attachment", "admin_log", "crud_log"} {
		if err := verifyOwnerColumnSchema(db, config, logical); err != nil {
			return err
		}
	}
	return verifyCountryDictionaryContract(db, config)
}

func verifyFinalDataContractImpl(db *gorm.DB, config *conf.Configuration) error {
	if err := validateMigrationOwners(db, core.TableName(config, "user"), core.TableName(config, "admin")); err != nil {
		return err
	}
	if err := validateLogOwnerMatchesUser(db, core.TableName(config, "user_money_log"), core.TableName(config, "user")); err != nil {
		return err
	}
	if err := verifySecuritySeedIdentity(db, config); err != nil {
		return err
	}
	if err := verifyCountryMenuData(db, config); err != nil {
		return err
	}
	return verifyUploadConfigData(db, config)
}

func verifyUserTableContract(db *gorm.DB, config *conf.Configuration) error {
	table := core.TableName(config, "user")
	for _, column := range []string{"id", "admin_id", "username", "nickname", "avatar", "email", "mobile", "password", "status", "money", "last_login_time", "last_login_ip", "login_failure", "join_ip", "join_time", "create_time", "update_time"} {
		if err := requireColumn(db, table, column); err != nil {
			return err
		}
	}
	for _, column := range []string{"gender", "birthday", "score", "motto", "group_id", "salt"} {
		if core.ColumnExists(db, table, column) {
			return fmt.Errorf("%s.%s is not part of the final user contract", table, column)
		}
	}
	return requireIndexColumns(db, table, "idx_admin_id", []string{"admin_id"})
}

func verifyAdminTableContract(db *gorm.DB, config *conf.Configuration) error {
	table := core.TableName(config, "admin")
	if core.ColumnExists(db, table, "salt") {
		return fmt.Errorf("%s.salt is not part of the final admin contract", table)
	}
	return nil
}

func verifyStatusContract(db *gorm.DB, config *conf.Configuration) error {
	for _, logical := range []string{"admin", "user"} {
		table := core.TableName(config, logical)
		def, ok, err := core.MigrationColumnInfo(db, table, "status")
		if err != nil {
			return err
		}
		if !ok || !strings.Contains(strings.ToLower(def.ColumnType), "varchar") || !strings.EqualFold(def.Nullable, "NO") {
			return fmt.Errorf("%s.status protocol schema invalid", table)
		}
		var invalid int64
		if err := db.Raw("SELECT COUNT(*) FROM " + core.QuoteIdentifier(table) + " WHERE status IS NULL OR BINARY status NOT IN ('enable','disable')").Scan(&invalid).Error; err != nil {
			return err
		}
		if invalid != 0 {
			return fmt.Errorf("%s.status contains invalid values", table)
		}
	}
	return nil
}

func verifyMoneyDecimalContract(db *gorm.DB, config *conf.Configuration) error {
	for _, item := range userMoneyDecimalColumns {
		table := core.TableName(config, item.table)
		def, ok, err := core.MigrationColumnInfo(db, table, item.column)
		if err != nil {
			return err
		}
		if !ok || strings.ToLower(def.ColumnType) != userMoneyDecimalType || !strings.EqualFold(def.Nullable, "NO") {
			return fmt.Errorf("%s.%s has invalid money schema", table, item.column)
		}
	}
	return nil
}

func verifyOwnerColumnSchema(db *gorm.DB, config *conf.Configuration, logical string) error {
	table := core.TableName(config, logical)
	def, ok, err := core.MigrationColumnInfo(db, table, "admin_id")
	if err != nil {
		return err
	}
	if !ok || !validOwnerColumn(def) {
		return fmt.Errorf("%s.admin_id has invalid owner schema", table)
	}
	return requireIndexColumns(db, table, "idx_admin_id", []string{"admin_id"})
}

func verifyCountryDictionaryContract(db *gorm.DB, config *conf.Configuration) error {
	for _, table := range []string{"country_language", "country_language_content", "country_currency"} {
		if err := requireTable(db, core.TableName(config, table)); err != nil {
			return err
		}
	}
	for _, item := range []struct{ table, column string }{
		{"country_language", "lan"}, {"country_language", "name"}, {"country_language", "remark"}, {"country_language", "status"}, {"country_language", "weigh"},
		{"country_language_content", "lan"}, {"country_language_content", "group"}, {"country_language_content", "key"}, {"country_language_content", "type"}, {"country_language_content", "value"},
		{"country_currency", "code"}, {"country_currency", "name"}, {"country_currency", "symbol"}, {"country_currency", "rate"}, {"country_currency", "status"}, {"country_currency", "weigh"},
	} {
		if err := requireColumn(db, core.TableName(config, item.table), item.column); err != nil {
			return err
		}
	}
	for _, index := range []struct {
		table, name string
		columns     []string
	}{
		{core.TableName(config, "country_language"), "uk_country_language_lan", []string{"lan"}},
		{core.TableName(config, "country_language_content"), "uk_country_language_content_lan_group_key", []string{"lan", "group", "key"}},
		{core.TableName(config, "country_currency"), "uk_country_currency_code", []string{"code"}},
	} {
		if err := requireIndexColumns(db, index.table, index.name, index.columns); err != nil {
			return err
		}
	}
	for _, table := range []string{"country_language", "country_currency"} {
		var invalid int64
		if err := db.Raw("SELECT COUNT(*) FROM " + core.QuoteIdentifier(core.TableName(config, table)) + " WHERE status NOT IN (0, 1) OR status IS NULL").Scan(&invalid).Error; err != nil {
			return err
		}
		if invalid != 0 {
			return fmt.Errorf("%s.status contains invalid values", table)
		}
	}
	var enabledLanguages int64
	if err := db.Raw("SELECT COUNT(*) FROM " + core.QuoteIdentifier(core.TableName(config, "country_language")) + " WHERE status = 1").Scan(&enabledLanguages).Error; err != nil {
		return err
	}
	if enabledLanguages == 0 {
		return fmt.Errorf("%s has no enabled language", core.TableName(config, "country_language"))
	}
	return nil
}

func verifyCountryMenuData(db *gorm.DB, config *conf.Configuration) error {
	table := core.QuoteIdentifier(core.TableName(config, "admin_rule"))
	names := []string{"country", "country/currency", "country/currency/index", "country/currency/add", "country/currency/edit", "country/currency/del", "country/currency/sortable", "country/language", "country/language/index", "country/language/add", "country/language/edit", "country/language/del", "country/language/sortable", "country/languageContent", "country/languageContent/index", "country/languageContent/add", "country/languageContent/edit", "country/languageContent/del", "country/languageContent/sortable"}
	for _, name := range names {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM "+table+" WHERE name = ?", name).Scan(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("country menu %s count=%d", name, count)
		}
	}
	return nil
}

func verifyUploadConfigData(db *gorm.DB, config *conf.Configuration) error {
	table := core.QuoteIdentifier(core.TableName(config, "config"))
	for _, name := range []string{"upload_mode", "upload_bucket", "upload_access_id", "upload_secret_key", "upload_url", "upload_cdn_url"} {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM "+table+" WHERE name = ? AND `group` = 'upload'", name).Scan(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("upload config %s count=%d", name, count)
		}
	}
	var groupValue string
	if err := db.Raw("SELECT value FROM " + table + " WHERE name = 'config_group'").Scan(&groupValue).Error; err != nil {
		return err
	}
	if !strings.Contains(groupValue, `"key":"upload"`) {
		return fmt.Errorf("config_group is missing upload group")
	}
	return nil
}

func validOwnerColumn(def core.MigrationColumn) bool {
	typ := strings.ToLower(def.ColumnType)
	return strings.Contains(typ, "int") && strings.Contains(typ, "unsigned") && strings.EqualFold(def.Nullable, "NO") && def.Default != nil && *def.Default == "0"
}
