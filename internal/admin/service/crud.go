package service

import (
	"context"
	"path/filepath"

	"buildadmin-go/internal/admin/dto"
	"buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/crudgen"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/util"
)

// CrudService 编排 CRUD 设计器的生成与删除：root 管理员校验、生成器调用与
// 归属标识缓存失效。文件与表结构变更只允许 root，规则集中在此。
type CrudService struct {
	tableM   *repository.TableRepository
	crudLogM *repository.CrudLogRepository
	config   *conf.Configuration
}

func NewCrudService(tableM *repository.TableRepository, crudLogM *repository.CrudLogRepository, config *conf.Configuration) *CrudService {
	return &CrudService{tableM: tableM, crudLogM: crudLogM, config: config}
}

// Generate 执行 CRUD 代码与表结构生成。register/unregister 由调用方注入
// 原子路由钩子，service 不依赖 gin 与全局中间件。
//
// 设计器提交的表设计在生成前写回 crud_specs/<table>.yaml：spec 是唯一事实源，
// 否则设计器生成的表对 crud:apply、migrate --plan 与全新环境重建都不可见。
// 生成失败时回滚该文件，避免留下没有对应产物的孤儿 spec。
func (s *CrudService) Generate(_ context.Context, actor data_scope.Actor, p dto.CrudGenerateParam, register, unregister func(method, route string)) error {
	if err := requireCrudRootActor(actor); err != nil {
		return err
	}
	if _, err := crudgen.GenerateFromSpec(s.tableM.DB(), s.config, s.tableM, crudgen.GenerateOptions{
		Table:                 p.Table,
		Fields:                p.Fields,
		Type:                  p.Type,
		AdminID:               actor.AdminID,
		AllowRebuild:          true, // 设计器前端 generateCheck 已弹窗确认删表重建
		SpecOutputDir:         specDir(),
		RegisterAtomicRoute:   register,
		UnregisterAtomicRoute: unregister,
	}); err != nil {
		return err
	}
	data_scope.InvalidateBusinessIdentifierCache()
	return nil
}

// specDir 返回 spec 目录：优先仓库内 crud_specs/，缺失时按其标准位置新建。
func specDir() string {
	if dir := crudgen.DefaultSpecDir(); dir != "" {
		return dir
	}
	return filepath.Join(util.RootPath(), "crud_specs")
}

// Delete 删除 CRUD 生成产物、菜单记录与 crud_log 行；删除同样只允许 root。
func (s *CrudService) Delete(ctx context.Context, actor data_scope.Actor, id int32, unregister func(method, route string)) error {
	if err := requireCrudRootActor(actor); err != nil {
		return err
	}
	crudLog, err := s.crudLogM.GetOneWithActor(ctx, actor, id)
	if err != nil {
		return err
	}
	if err := crudgen.DeleteFromSpecWithHooks(s.tableM.DB(), s.config, crudLog.Tablename, unregister); err != nil {
		return err
	}
	data_scope.InvalidateBusinessIdentifierCache()
	return nil
}

// requireCrudRootActor 是生成/删除的唯一授权判据：文件与表结构变更属超管
// 专属，非 root（含缺失 actor 的零值）一律拒绝。
func requireCrudRootActor(actor data_scope.Actor) error {
	if !actor.Unrestricted {
		return cErr.ForbiddenRequest("CRUD file and schema changes require a root administrator")
	}
	return nil
}
