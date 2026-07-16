package model

import (
	"fmt"
	"go-build-admin/app/admin/model/simple"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Attachment 附件表
type Attachment struct {
	ID             int32        `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`      // ID
	Topic          string       `gorm:"column:topic;not null;comment:细目" json:"topic"`                     // 细目
	AdminID        int32        `gorm:"column:admin_id;not null;comment:上传管理员ID" json:"admin_id"`          // 上传管理员ID
	UserID         int32        `gorm:"column:user_id;not null;comment:上传用户ID" json:"user_id"`             // 上传用户ID
	URL            string       `gorm:"column:url;not null;comment:物理路径" json:"url"`                       // 物理路径
	Width          int32        `gorm:"column:width;not null;comment:宽度" json:"width"`                     // 宽度
	Height         int32        `gorm:"column:height;not null;comment:高度" json:"height"`                   // 高度
	Name           string       `gorm:"column:name;not null;comment:原始名称" json:"name"`                     // 原始名称
	Size           int32        `gorm:"column:size;not null;comment:大小" json:"size"`                       // 大小
	Mimetype       string       `gorm:"column:mimetype;not null;comment:mime类型" json:"mimetype"`           // mime类型
	Quote          int32        `gorm:"column:quote;not null;comment:上传(引用)次数" json:"quote"`               // 上传(引用)次数
	Storage        string       `gorm:"column:storage;not null;comment:存储方式" json:"storage"`               // 存储方式
	Sha1           string       `gorm:"column:sha1;not null;comment:sha1编码" json:"sha1"`                   // sha1编码
	CreateTime     int64        `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
	LastUploadTime int64        `gorm:"column:last_upload_time;comment:最后上传时间" json:"last_upload_time"`    // 最后上传时间
	Suffix         string       `gorm:"-" json:"suffix"`
	FullUrl        string       `gorm:"-" json:"full_url"`
	Admin          simple.Admin `gorm:"foreignKey:AdminID" json:"admin"`
	User           simple.User  `gorm:"foreignKey:UserID" json:"user"`
}

type AttachmentModel struct {
	BaseModel
	config   *conf.Configuration
	enforcer data_scope.Enforcer
	Policy   data_scope.ResourcePolicy
}

func NewAttachmentModel(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *AttachmentModel {
	return &AttachmentModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "attachment",
			Key:              "id",
			QuickSearchField: "name",
			sqlDB:            sqlDB,
		},
		config:   config,
		enforcer: enforcer,
		Policy:   data_scope.ResourcePolicy{Mode: data_scope.ModeRequired, OwnerColumn: "admin_id", AssignOnCreate: true},
	}
}

func (s *AttachmentModel) scoped(ctx *gin.Context, db *gorm.DB) *gorm.DB {
	if s.enforcer == nil {
		tx := db.Session(&gorm.Session{})
		tx.AddError(data_scope.ErrScopedAccessDenied)
		return tx
	}
	return s.enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: "attachment", Column: "admin_id"})
}

func (s *AttachmentModel) GetOne(ctx *gin.Context, id int32) (attachment Attachment, err error) {
	err = s.scoped(ctx, s.sqlDB.Table(s.TableName+" AS attachment")).Where("attachment.id=?", id).Preload("Admin").Preload("User").Take(&attachment).Error
	if err == nil {
		_, err = s.DealData(ctx, &attachment)
	}
	return
}

func (s *AttachmentModel) DealData(ctx *gin.Context, data *Attachment) (*Attachment, error) {
	data.Suffix = strings.TrimLeft(filepath.Ext(data.URL), ".")
	data.FullUrl = utils.FullUrl(data.URL, s.config.App.CdnUrl, utils.GetBaseURL(ctx), "")
	return data, nil
}

func (s *AttachmentModel) List(ctx *gin.Context) (list []*Attachment, total int64, err error) {
	tableInfo := s.TableInfo()
	// The query uses an explicit alias so the owner predicate remains
	// unambiguous alongside Admin/User joins.
	tableInfo.TableName = "attachment"
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, tableInfo, nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.scoped(ctx, s.sqlDB.Table(s.TableName+" AS attachment")).Model(&Attachment{}).Preload("Admin").Preload("User").
		Joins("Admin").
		Joins("User").
		Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	for _, v := range list {
		if _, err = s.DealData(ctx, v); err != nil {
			return nil, 0, err
		}
	}
	return
}

func (s *AttachmentModel) Edit(ctx *gin.Context, data Attachment) error {
	updates := map[string]interface{}{"topic": data.Topic, "url": data.URL, "width": data.Width, "height": data.Height, "name": data.Name, "size": data.Size, "mimetype": data.Mimetype, "quote": data.Quote, "storage": data.Storage, "sha1": data.Sha1}
	tx := s.scoped(ctx, s.sqlDB.Table(s.TableName+" AS attachment")).Where("attachment.id = ?", data.ID).Updates(updates)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *AttachmentModel) Del(ctx *gin.Context, ids interface{}) error {
	values, ok := ids.([]int32)
	if !ok || len(values) == 0 {
		return fmt.Errorf("invalid attachment ids")
	}
	seen := make(map[int32]struct{}, len(values))
	normalized := make([]int32, 0, len(values))
	for _, id := range values {
		if id <= 0 {
			return fmt.Errorf("invalid attachment id %d", id)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			normalized = append(normalized, id)
		}
	}
	var list []Attachment
	err := s.sqlDB.Transaction(func(tx *gorm.DB) error {
		scoped := s.scoped(ctx, tx.Table(s.TableName+" AS attachment"))
		if err := scoped.Where("attachment.id IN ?", normalized).Find(&list).Error; err != nil {
			return err
		}
		if len(list) != len(normalized) {
			return gorm.ErrRecordNotFound
		}
		del := scoped.Where("attachment.id IN ?", normalized).Delete(&Attachment{})
		if del.Error != nil {
			return del.Error
		}
		if del.RowsAffected != int64(len(normalized)) {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, v := range list {
		if utils.PathExists(utils.RootPath() + v.URL) {
			if removeErr := os.Remove(utils.RootPath() + v.URL); removeErr != nil {
				return removeErr
			}
		}
	}
	return nil
}
