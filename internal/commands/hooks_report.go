package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"buildadmin-go/internal/pkg/crudhooks"

	"go.uber.org/zap"
)

// unregisteredMutationHooks 返回 spec 声明了 hooks.afterMutation、但当前进程
// 还没注册实现的条目（"表 -> 钩子名"）。serve 启动时用它把漏注册暴露在日志里，
// 否则只会在第一次写操作时打一条 no-op 告警。
func unregisteredMutationHooks(specDir string) ([]string, error) {
	if specDir == "" {
		return nil, nil
	}
	paths := collectSpecPaths(specDir)
	missing := make([]string, 0)
	for _, path := range paths {
		options, err := helper.LoadSpec(path)
		if err != nil {
			return nil, err
		}
		if options.Table.Hooks == nil || options.Table.Hooks.AfterMutation == "" {
			continue
		}
		name := options.Table.Hooks.AfterMutation
		if crudhooks.RegisteredMutation(name) {
			continue
		}
		missing = append(missing, options.Table.Name+" -> "+name)
	}
	return missing, nil
}

// warnUnregisteredMutationHooks 输出启动期告警；读 spec 失败只记录不阻断启动。
func warnUnregisteredMutationHooks(logger *zap.Logger) {
	missing, err := unregisteredMutationHooks(helper.DefaultSpecDir())
	if err != nil {
		if logger != nil {
			logger.Warn("CRUD hook report skipped: cannot read crud_specs", zap.Error(err))
		}
		return
	}
	if logger == nil {
		return
	}
	for _, entry := range missing {
		logger.Warn("CRUD hook declared in spec but not registered; mutation side effects will be skipped", zap.String("hook", entry))
	}
}
