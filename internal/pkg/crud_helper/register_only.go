package crud_helper

import (
	"fmt"
)

// registerOnlyFromSpec 处理受保护核心表的"仅登记不生成"（registerOnly）模式：
//
// 受保护核心表（user、user_money_log 等）不参与 CRUD 代码生成（其 repo/handler
// 由框架手写维护）。registerOnly 的声明事实源是 crud_specs/ 目录本身——spec
// 文件存在即登记生效：
//   - validateInheritParent 直接从 crud_specs/<parent>.yaml 读取父表声明；
//   - cascade:sync 通过 ScanInheritDeclarations 从 crud_specs/*.yaml 聚合；
//   - crud:apply 对 registerOnly spec 静默跳过（不建表不改表）。
//
// 因此本路径**不写 crud_log、不生成任何文件、不注册锚点**：spec 合法即成功
//（幂等，可重复执行；重新安装后无需逐表跑生成）。仅做三项校验：
//   - 目标必须是受保护核心表（登记是核心表的特权）；
//   - 必须声明 dataScope.reassignable（父表登记）或 dataScope.inheritFrom
//     （子表登记）之一——登记的目的就是参与级联；
//   - spec 结构与字段集合法（与普通生成同一套校验）。
//
// GenerateFromSpec 在 IsProtectedTable 检查之前分流：registerOnly + 非受保护表
// 直接拒绝。纯配置/文件系统校验，无数据库依赖。
func registerOnlyFromSpec(opts GenerateOptions) (*GenerateResult, error) {
	// 防御性复核：登记是受保护核心表的特权（GenerateFromSpec 入口已分流，
	// 此处自洽保证函数级调用同样 fail-closed；纯字符串检查无需依赖）。
	if !IsProtectedTable(opts.Table.Name) {
		return nil, fmt.Errorf("registerOnly is reserved for protected core tables; %q is not protected", opts.Table.Name)
	}
	// 登记形态校验（纯配置检查，无需依赖，前置 fail-fast）。
	ds := opts.Table.DataScope
	if ds == nil || (!ds.Reassignable && ds.InheritFrom == nil) {
		return nil, fmt.Errorf("registerOnly spec %q must declare dataScope.reassignable (parent registration) or dataScope.inheritFrom (child registration)", opts.Table.Name)
	}
	if err := normalizeTableConfiguration(&opts.Table); err != nil {
		return nil, err
	}
	if err := ValidateGenerationInput(opts.Table, opts.Fields); err != nil {
		return nil, err
	}
	// 子表登记形态下校验父表 spec 已声明 reassignable（fail-closed；spec 事实源，
	// 不查 crud_log——重新安装后 crud_log 为空也能通过）。
	if ds.InheritFrom != nil {
		if err := validateInheritParent(DefaultSpecsDir(), ds.InheritFrom.Table); err != nil {
			return nil, err
		}
	}
	return &GenerateResult{}, nil
}
