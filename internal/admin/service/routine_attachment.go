package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/common/upload"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/util"

	"gorm.io/gorm"
)

// RoutineAttachmentService 承载附件删除的业务编排：引用计数（quote）语义、
// 事务内 scoped 删除与提交后的本地/OSS 文件清理。repo 只保留 scoped 锁定
// 与原子写原语。
type RoutineAttachmentService struct {
	attachmentM *adminmodel.AttachmentRepository
	config      *conf.Configuration
}

func NewRoutineAttachmentService(attachmentM *adminmodel.AttachmentRepository, config *conf.Configuration) *RoutineAttachmentService {
	return &RoutineAttachmentService{attachmentM: attachmentM, config: config}
}

// Del 删除一批附件：仍被引用的（quote>1）只减计数，最后一次引用才物理
// 删除行并清理文件；任何越权或缺失的 id 让整批原子回退。
func (s *RoutineAttachmentService) Del(ctx context.Context, ids []int32, actor data_scope.Actor) error {
	normalized, err := normalizeAttachmentIDs(ids)
	if err != nil {
		return err
	}
	if data_scope.ValidateActor(actor) != nil {
		return data_scope.ErrScopedAccessDenied
	}
	var physicallyDeleted []upload.Attachment
	err = s.attachmentM.Transaction(ctx, func(tx *gorm.DB) error {
		list, err := s.attachmentM.LockScopedWithActor(ctx, tx, normalized, actor)
		if err != nil {
			return err
		}
		for _, attachment := range list {
			if attachment.Quote > 1 {
				if err := s.attachmentM.DecrementQuoteTx(tx, attachment.ID); err != nil {
					return err
				}
				continue
			}
			if err := s.attachmentM.DeleteRowTx(tx, attachment.ID); err != nil {
				return err
			}
			physicallyDeleted = append(physicallyDeleted, attachment)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, v := range physicallyDeleted {
		if v.Storage == "alioss" {
			if err := upload.NewAliossStorage(s.attachmentM.DB(), s.config).Delete(v.URL); err != nil {
				return err
			}
		} else {
			paths := []string{
				filepath.Join(util.RootPath(), "public", strings.TrimLeft(v.URL, "/")),
				filepath.Join(util.RootPath(), strings.TrimLeft(v.URL, "/")),
			}
			for _, path := range paths {
				if !util.PathExists(path) {
					continue
				}
				if removeErr := os.Remove(path); removeErr != nil {
					return removeErr
				}
				break
			}
		}
	}
	return nil
}

// normalizeAttachmentIDs validates and de-duplicates a batch of attachment ids.
func normalizeAttachmentIDs(ids []int32) ([]int32, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("invalid attachment ids")
	}
	seen := make(map[int32]struct{}, len(ids))
	normalized := make([]int32, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("invalid attachment id %d", id)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			normalized = append(normalized, id)
		}
	}
	return normalized, nil
}
