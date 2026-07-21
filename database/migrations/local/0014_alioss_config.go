package local

import (
	"encoding/json"
	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"go-build-admin/database/migrations/model"
	"gorm.io/gorm"
)

func version0014(db *gorm.DB, config *conf.Configuration) error {
	table := core.TableName(config, "config")
	if !core.TableExists(db, table) {
		return nil
	}
	var group model.Config
	if err := db.Table(table).Where("name = ?", "config_group").Take(&group).Error; err == nil {
		var items []map[string]string
		if json.Unmarshal([]byte(group.Value), &items) == nil {
			seen := false
			for _, item := range items {
				if item["key"] == "upload" {
					seen = true
					break
				}
			}
			if !seen {
				items = append(items, map[string]string{"key": "upload", "value": "Upload"})
				raw, _ := json.Marshal(items)
				if err := db.Table(table).Where("id = ?", group.ID).Update("value", string(raw)).Error; err != nil {
					return err
				}
			}
		}
	}
	rows := aliossConfigRows()
	for _, row := range rows {
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

func aliossConfigRows() []model.Config {
	return []model.Config{
		{Name: "upload_mode", Group: "upload", Title: "存储方式", Type: "select", Value: "local", Content: `{"local":"本地磁盘存储","alioss":"阿里云对象存储OSS"}`, Rule: "required", Weigh: 99},
		{Name: "upload_bucket", Group: "upload", Title: "Bucket名称", Tip: "请在阿里云对象存储控制台查询", Type: "string", Value: "", Rule: "", Weigh: 98},
		{Name: "upload_access_id", Group: "upload", Title: "AccessKey ID", Tip: "请在阿里云个人中心查询", Type: "string", Value: "", Rule: "", Weigh: 97},
		{Name: "upload_secret_key", Group: "upload", Title: "AccessKey Secret", Tip: "请在阿里云个人中心查询", Type: "string", Value: "", Rule: "", Weigh: 96},
		{Name: "upload_url", Group: "upload", Title: "存储区域", Tip: "请选择存储区域", Type: "select", Value: "", Content: `{"oss-cn-hangzhou":"华东1（杭州） oss-cn-hangzhou","oss-cn-shanghai":"华东2（上海） oss-cn-shanghai","oss-cn-nanjing":"华东5（南京本地地域） oss-cn-nanjing","oss-cn-fuzhou":"华东6（福州本地地域） oss-cn-fuzhou","oss-cn-qingdao":"华北1（青岛） oss-cn-qingdao","oss-cn-beijing":"华北2（北京） oss-cn-beijing","oss-cn-zhangjiakou":"华北 3（张家口） oss-cn-zhangjiakou","oss-cn-huhehaote":"华北5（呼和浩特） oss-cn-huhehaote","oss-cn-wulanchabu":"华北6（乌兰察布） oss-cn-wulanchabu","oss-cn-shenzhen":"华南1（深圳） oss-cn-shenzhen","oss-cn-heyuan":"华南2（河源） oss-cn-heyuan","oss-cn-guangzhou":"华南3（广州） oss-cn-guangzhou","oss-cn-chengdu":"西南1（成都） oss-cn-chengdu","oss-cn-hongkong":"中国（香港） oss-cn-hongkong","oss-us-west-1":"美国（硅谷） oss-us-west-1","oss-us-east-1":"美国（弗吉尼亚） oss-us-east-1","oss-ap-northeast-1":"日本（东京） oss-ap-northeast-1","oss-ap-northeast-2":"韩国（首尔） oss-ap-northeast-2","oss-ap-southeast-1":"新加坡 oss-ap-southeast-1","oss-ap-southeast-2":"澳大利亚（悉尼） oss-ap-southeast-2","oss-ap-southeast-3":"马来西亚（吉隆坡） oss-ap-southeast-3","oss-ap-southeast-5":"印度尼西亚（雅加达） oss-ap-southeast-5","oss-ap-southeast-6":"菲律宾（马尼拉） oss-ap-southeast-6","oss-ap-southeast-7":"泰国（曼谷） oss-ap-southeast-7","oss-ap-south-1":"印度（孟买） oss-ap-south-1","oss-eu-central-1":"德国（法兰克福） oss-eu-central-1","oss-eu-west-1":"英国（伦敦） oss-eu-west-1","oss-me-east-1":"阿联酋（迪拜） oss-me-east-1","oss-cn-hzjbp":"华东1金融云 oss-cn-hzjbp","oss-cn-shanghai-finance-1":"华东2金融云 oss-cn-shanghai-finance-1","oss-cn-beijing-finance-1":"华北2金融云 oss-cn-beijing-finance-1","oss-cn-shenzhen-finance-1":"华南1金融云 oss-cn-shenzhen-finance-1","oss-cn-hzfinance":"杭州金融云公网 oss-cn-hzfinance","oss-cn-shanghai-finance-1-pub":"上海金融云公网 oss-cn-shanghai-finance-1-pub","oss-cn-szfinance":"深圳金融云公网 oss-cn-szfinance","oss-cn-beijing-finance-1-pub":"北京金融云公网 oss-cn-beijing-finance-1-pub"}`, Rule: "", Weigh: 95},
		{Name: "upload_cdn_url", Group: "upload", Title: "CDN地址", Tip: "请输入阿里云对象存储的CDN加速域名，以http(s)://开头，比如：https://example.com", Type: "string", Value: "", Rule: "", Weigh: 94},
	}
}
