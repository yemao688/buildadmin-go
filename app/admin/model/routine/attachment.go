package routine

import (
	"fmt"
	commonModel "go-build-admin/app/common/model"
	"go-build-admin/app/common/upload"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AttachmentModel struct {
	commonModel.BaseModel
	config   *conf.Configuration
	enforcer data_scope.Enforcer
	Policy   data_scope.ResourcePolicy
}

func NewAttachmentModel(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *AttachmentModel {
	return &AttachmentModel{
		BaseModel: commonModel.NewBaseModel(config.Database.Prefix+"attachment", "id", "name", sqlDB),
		config:    config,
		enforcer:  enforcer,
		Policy:    data_scope.ResourcePolicy{Mode: data_scope.ModeRequired, OwnerColumn: "admin_id", AssignOnCreate: true},
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

func (s *AttachmentModel) GetOne(ctx *gin.Context, id int32) (attachment upload.Attachment, err error) {
	err = s.scoped(ctx, s.DB().Table(s.TableName+" AS attachment")).Where("attachment.id=?", id).Preload("Admin").Preload("User").Take(&attachment).Error
	if err == nil {
		_, err = s.DealData(ctx, &attachment)
	}
	return
}

func (s *AttachmentModel) DealData(ctx *gin.Context, data *upload.Attachment) (*upload.Attachment, error) {
	data.Suffix = strings.TrimLeft(filepath.Ext(data.URL), ".")
	if data.Storage == "alioss" {
		data.FullUrl = upload.NewAliossStorage(s.DB(), s.config).URL(data.URL)
	} else {
		data.FullUrl = utils.FullUrl(data.URL, s.config.App.CdnUrl, utils.GetBaseURL(ctx), "")
	}
	return data, nil
}

func (s *AttachmentModel) List(ctx *gin.Context) (list []*upload.Attachment, total int64, err error) {
	tableInfo := s.TableInfo()
	// The query uses an explicit alias so the owner predicate remains
	// unambiguous alongside Admin/User joins.
	tableInfo.TableName = "attachment"
	whereS, whereP, orderS, limit, offset, err := commonModel.QueryBuilder(ctx, tableInfo, nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.scoped(ctx, s.DB().Table(s.TableName+" AS attachment")).Model(&upload.Attachment{}).Preload("Admin").Preload("User").
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

func (s *AttachmentModel) Edit(ctx *gin.Context, data upload.Attachment) error {
	updates := map[string]interface{}{"topic": data.Topic, "url": data.URL, "width": data.Width, "height": data.Height, "name": data.Name, "size": data.Size, "mimetype": data.Mimetype, "quote": data.Quote, "storage": data.Storage, "sha1": data.Sha1}
	tx := s.scoped(ctx, s.DB().Table(s.TableName+" AS attachment")).Where("attachment.id = ?", data.ID).Updates(updates)
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
	var list []upload.Attachment
	err := s.Transaction(ctx, func(tx *gorm.DB) error {
		scoped := s.scoped(ctx, tx.Table(s.TableName+" AS attachment"))
		if err := scoped.Where("attachment.id IN ?", normalized).Find(&list).Error; err != nil {
			return err
		}
		if len(list) != len(normalized) {
			return gorm.ErrRecordNotFound
		}
		del := scoped.Where("attachment.id IN ?", normalized).Delete(&upload.Attachment{})
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
		if v.Storage == "alioss" {
			if err := upload.NewAliossStorage(s.DB(), s.config).Delete(v.URL); err != nil {
				return err
			}
		} else if utils.PathExists(utils.RootPath() + v.URL) {
			if removeErr := os.Remove(utils.RootPath() + v.URL); removeErr != nil {
				return removeErr
			}
		}
	}
	return nil
}
