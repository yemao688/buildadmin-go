package repository

import (
	"context"
	"buildadmin-go/internal/common/upload"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/data_scope"
	persistence "buildadmin-go/internal/pkg/persistence"
	"buildadmin-go/internal/utils"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AttachmentRepository struct {
	persistence.BaseModel
	config   *conf.Configuration
	enforcer data_scope.Enforcer
	Policy   data_scope.ResourcePolicy
}

func NewAttachmentRepository(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *AttachmentRepository {
	return &AttachmentRepository{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"attachment", "id", "name", sqlDB),
		config:    config,
		enforcer:  enforcer,
		Policy:    data_scope.ResourcePolicy{Mode: data_scope.ModeRequired, OwnerColumn: "admin_id", AssignOnCreate: true},
	}
}

func (s *AttachmentRepository) scoped(ctx *gin.Context, db *gorm.DB) *gorm.DB {
	if s.enforcer == nil {
		tx := db.Session(&gorm.Session{})
		tx.AddError(data_scope.ErrScopedAccessDenied)
		return tx
	}
	return s.enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: "attachment", Column: "admin_id"})
}

func (s *AttachmentRepository) GetOne(ctx *gin.Context, id int32) (attachment upload.Attachment, err error) {
	err = s.scoped(ctx, s.DB().Table(s.TableName+" AS attachment")).Where("attachment.id=?", id).Preload("Admin").Preload("User").Take(&attachment).Error
	if err == nil {
		_, err = s.DealData(ctx, &attachment)
	}
	return
}

func (s *AttachmentRepository) DealData(ctx *gin.Context, data *upload.Attachment) (*upload.Attachment, error) {
	data.Suffix = strings.TrimLeft(filepath.Ext(data.URL), ".")
	if data.Storage == "alioss" {
		data.FullUrl = upload.NewAliossStorage(s.DB(), s.config).URL(data.URL)
	} else {
		data.FullUrl = utils.FullUrl(data.URL, s.config.App.CdnUrl, utils.GetBaseURL(ctx), "")
	}
	return data, nil
}

func (s *AttachmentRepository) List(ctx *gin.Context) (list []*upload.Attachment, total int64, err error) {
	tableInfo := s.TableInfo()
	// The query uses an explicit alias so the owner predicate remains
	// unambiguous alongside Admin/User joins.
	tableInfo.TableName = "attachment"
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, tableInfo, nil)
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

func (s *AttachmentRepository) Edit(ctx *gin.Context, data upload.Attachment) error {
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

// LockScopedWithActor loads the attachments FOR UPDATE inside the actor's
// scope; a missing or out-of-scope id fails with gorm.ErrRecordNotFound so
// the batch is all-or-nothing.
func (s *AttachmentRepository) LockScopedWithActor(ctx context.Context, tx *gorm.DB, ids []int32, actor data_scope.Actor) ([]upload.Attachment, error) {
	var list []upload.Attachment
	scoped := s.scopedWithActor(ctx, tx.Table(s.TableName+" AS attachment"), actor)
	if err := scoped.Clauses(clause.Locking{Strength: "UPDATE"}).Where("attachment.id IN ?", ids).Find(&list).Error; err != nil {
		return nil, err
	}
	if len(list) != len(ids) {
		return nil, gorm.ErrRecordNotFound
	}
	return list, nil
}

// scopedWithActor is the actor-explicit counterpart of scoped.
func (s *AttachmentRepository) scopedWithActor(ctx context.Context, db *gorm.DB, actor data_scope.Actor) *gorm.DB {
	if s.enforcer == nil {
		tx := db.Session(&gorm.Session{})
		tx.AddError(data_scope.ErrScopedAccessDenied)
		return tx
	}
	enforcer, ok := s.enforcer.(interface {
		ScopeWithActor(context.Context, *gorm.DB, data_scope.Actor, data_scope.OwnerRef) *gorm.DB
	})
	if !ok {
		tx := db.Session(&gorm.Session{})
		tx.AddError(data_scope.ErrScopedAccessDenied)
		return tx
	}
	return enforcer.ScopeWithActor(ctx, db, actor, data_scope.OwnerRef{TableAlias: "attachment", Column: "admin_id"})
}

// DecrementQuoteTx drops the reference count by one inside the caller's
// transaction (atomic write primitive).
func (s *AttachmentRepository) DecrementQuoteTx(tx *gorm.DB, id int32) error {
	result := tx.Table(s.TableName).Where("id = ?", id).Update("quote", gorm.Expr("quote - 1"))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteRowTx deletes one attachment row inside the caller's transaction
// (atomic write primitive).
func (s *AttachmentRepository) DeleteRowTx(tx *gorm.DB, id int32) error {
	del := tx.Table(s.TableName).Where("id = ?", id).Delete(&upload.Attachment{})
	if del.Error != nil {
		return del.Error
	}
	if del.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
