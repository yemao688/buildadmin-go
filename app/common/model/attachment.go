package model

import (
	adminModel "go-build-admin/app/admin/model"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const TableNameAttachment = "ba_attachment"

// Attachment 附件表
type Attachment struct {
	ID             int32                  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`      // ID
	Topic          string                 `gorm:"column:topic;not null;comment:细目" json:"topic"`                     // 细目
	AdminID        int32                  `gorm:"column:admin_id;not null;comment:上传管理员ID" json:"admin_id"`          // 上传管理员ID
	UserID         int32                  `gorm:"column:user_id;not null;comment:上传用户ID" json:"user_id"`             // 上传用户ID
	URL            string                 `gorm:"column:url;not null;comment:物理路径" json:"url"`                       // 物理路径
	Width          int32                  `gorm:"column:width;not null;comment:宽度" json:"width"`                     // 宽度
	Height         int32                  `gorm:"column:height;not null;comment:高度" json:"height"`                   // 高度
	Name           string                 `gorm:"column:name;not null;comment:原始名称" json:"name"`                     // 原始名称
	Size           int32                  `gorm:"column:size;not null;comment:大小" json:"size"`                       // 大小
	Mimetype       string                 `gorm:"column:mimetype;not null;comment:mime类型" json:"mimetype"`           // mime类型
	Quote          int32                  `gorm:"column:quote;not null;comment:上传(引用)次数" json:"quote"`               // 上传(引用)次数
	Storage        string                 `gorm:"column:storage;not null;comment:存储方式" json:"storage"`               // 存储方式
	Sha1           string                 `gorm:"column:sha1;not null;comment:sha1编码" json:"sha1"`                   // sha1编码
	CreateTime     int64                  `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
	LastUploadTime int64                  `gorm:"column:last_upload_time;comment:最后上传时间" json:"last_upload_time"`    // 最后上传时间
	Suffix         string                 `gorm:"-" json:"suffix"`
	FullUrl        string                 `gorm:"-" json:"full_url"`
	Admin          adminModel.SimpleAdmin `gorm:"foreignKey:AdminID" json:"admin"`
	User           adminModel.SimpleUser  `gorm:"foreignKey:UserID" json:"user"`
}

func (*Attachment) TableName() string {
	return TableNameAttachment
}

type AttachmentModel struct {
	BaseModel
	config *conf.Configuration
}

func NewAttachmentModel(sqlDB *gorm.DB, config *conf.Configuration) *AttachmentModel {
	return &AttachmentModel{
		BaseModel: BaseModel{
			TableName:        TableNameAttachment,
			Key:              "id",
			QuickSearchField: "name",
			DataLimit:        "",
			sqlDB:            sqlDB,
		},
		config: config,
	}
}

func (s *AttachmentModel) GetOne(ctx *gin.Context, id int32) (attachment Attachment, err error) {
	err = s.sqlDB.Table(s.TableName).Where("id=?", id).Take(&attachment).Error
	s.DealData(ctx, &attachment)
	return
}

func (s *AttachmentModel) DealData(ctx *gin.Context, data *Attachment) (*Attachment, error) {
	data.Suffix = strings.TrimLeft(filepath.Ext(data.URL), ".")
	data.FullUrl = utils.FullUrl(data.URL, s.config.App.CdnUrl, utils.GetBaseURL(ctx), "")
	return data, nil
}

func (s *AttachmentModel) List(ctx *gin.Context) (list []*Attachment, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.sqlDB.Model(&Attachment{}).Preload("Admin").Preload("User").
		Joins("Admin").
		Joins("User").
		Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	for _, v := range list {
		s.DealData(ctx, v)
	}
	return
}

func (s *AttachmentModel) Edit(ctx *gin.Context, data Attachment) error {
	err := s.sqlDB.Table(s.TableName).Updates(data).Error
	return err
}

func (s *AttachmentModel) Del(ctx *gin.Context, ids interface{}) error {
	list := []Attachment{}
	if err := s.sqlDB.Table(s.TableName).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Find(&list).Error; err != nil {
		return err
	}

	for _, v := range list {
		if utils.PathExists(utils.RootPath() + v.URL) {
			os.Remove(utils.RootPath() + v.URL)
		}
	}

	err := s.sqlDB.Table(s.TableName).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}
