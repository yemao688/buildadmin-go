package model

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const TableNameAttachment = "ba_attachment"

// Attachment 附件表
type Attachment struct {
	ID             int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`   // ID
	Topic          string `gorm:"column:topic;not null;comment:细目" json:"topic"`                  // 细目
	AdminID        int32  `gorm:"column:admin_id;not null;comment:上传管理员ID" json:"admin_id"`       // 上传管理员ID
	UserID         int32  `gorm:"column:user_id;not null;comment:上传用户ID" json:"user_id"`          // 上传用户ID
	URL            string `gorm:"column:url;not null;comment:物理路径" json:"url"`                    // 物理路径
	Width          int32  `gorm:"column:width;not null;comment:宽度" json:"width"`                  // 宽度
	Height         int32  `gorm:"column:height;not null;comment:高度" json:"height"`                // 高度
	Name           string `gorm:"column:name;not null;comment:原始名称" json:"name"`                  // 原始名称
	Size           int32  `gorm:"column:size;not null;comment:大小" json:"size"`                    // 大小
	Mimetype       string `gorm:"column:mimetype;not null;comment:mime类型" json:"mimetype"`        // mime类型
	Quote          int32  `gorm:"column:quote;not null;comment:上传(引用)次数" json:"quote"`            // 上传(引用)次数
	Storage        string `gorm:"column:storage;not null;comment:存储方式" json:"storage"`            // 存储方式
	Sha1           string `gorm:"column:sha1;not null;comment:sha1编码" json:"sha1"`                // sha1编码
	CreateTime     int64  `gorm:"column:create_time;comment:创建时间" json:"create_time"`             // 创建时间
	LastUploadTime int64  `gorm:"column:last_upload_time;comment:最后上传时间" json:"last_upload_time"` // 最后上传时间
}

type AttachmentModel struct {
	BaseModel
	sqlDB *gorm.DB
}

func NewAttachmentModel(sqlDB *gorm.DB) *AttachmentModel {
	return &AttachmentModel{
		BaseModel: BaseModel{
			TableName:        TableNameAttachment,
			Key:              "id",
			QuickSearchField: "id",
			DataLimit:        "",
		},
		sqlDB: sqlDB}
}

func (s *AttachmentModel) Del(ctx *gin.Context, ids interface{}) error {
	err := s.sqlDB.Table(s.TableName).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}
