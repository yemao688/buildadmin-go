package i18n

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// legacyUnreferencedKeys 是框架代码不再引用、但保留下游兼容的历史消息：
// PHP 安装向导文案、旧邮件/短信流程与通用 CRUD 提示。它们不属于已删框架
// 功能（模块市场与绑定流程的键已在框架 3.5.2 清理），暂不随本次清理删除，
// 以免下游业务仍在使用时丢失译文；新增键不得再落入此清单。
var legacyUnreferencedKeys = map[string]struct{}{
	"Group Name Arr":                            {},
	"Dashboard remark":                          {},
	"Rule Type":                                 {},
	"Rule Title":                                {},
	"Rule Name":                                 {},
	"Change-field-name fail not exist":          {},
	"Del-field fail not exist":                  {},
	"Change-field-attr fail not exist":          {},
	"Add-field fail exist":                      {},
	"Please input correct username":             {},
	"Avatar modified successfully!":             {},
	"Attachment Remark":                         {},
	"{num} records and files have been deleted": {},
	"Variable name":                             {},
	"Test mail sent successfully~":              {},
	"Congratulations, receiving this email means that your email service has been configured correctly": {},
	"Data Table":                  {},
	"Primary Key":                 {},
	"No rows were restore":        {},
	"Data Fields":                 {},
	"No rows were rollback":       {},
	"User_id":                     {},
	"The user can't find it":      {},
	"Change note cannot be blank": {},
	"Score":                       {},
	"Failed to switch package manager. Please modify the configuration file manually:{content}": {},
	"Data table does not exist":                  {},
	"Login succeeded!":                           {},
	"Please enter the correct verification code": {},
	"Parameter {name} can not be empty":          {},
	"No rows were added":                         {},
	"No rows were deleted":                       {},
	"No rows updated":                            {},
	"Update successful":                          {},
	"Added successfully":                         {},
	"Parameter error":                            {},
	"Invalid collation because the weights of the two targets are equal":                  {},
	"File uploaded successfully":                                                          {},
	"No files were uploaded":                                                              {},
	"The uploaded image file is not a valid image":                                        {},
	"The uploaded file is too large ({min}MiB), Maximum file size:{max}MiB":               {},
	"No files have been uploaded or the file size exceeds the upload limit of the server": {},
	"Token login failed":                                                                  {},
	"Username is incorrect":                                                               {},
	"You are not logged in":                                                               {},
	"Old password error":                                                                  {},
	"Data updated successfully~":                                                          {},
	"nicknameChsDash":                                                                     {},
	"Password has been changed~":                                                          {},
	"Password has been changed, please login again~":                                      {},
	"Failed to modify password, please try again later~":                                  {},
	"{account} has been registered":                                                       {},
	"email format error":                                                                  {},
	"user_register":                                                                       {},
	"user_retrieve_pwd":                                                                   {},
	"user_change_email":                                                                   {},
	"user_email_verify":                                                                   {},
	"Your verification code is: {content}":                                                {},
	"Mail sent successfully~":                                                             {},
	"Account does not exist~":                                                             {},
	"Frequent email sending":                                                              {},
	"Email has been registered, please log in directly":                                   {},
	"The email has been occupied":                                                         {},
	"Email not registered":                                                                {},
	"Install the controller":                                                              {},
	"Click to see how to solve it":                                                        {},
	"Please check the config directory permissions":                                       {},
	"Please check the public directory permissions":                                       {},
	"The installation can continue, and some operations need to be completed manually": {},
	"Allow execution":                        {},
	"Allow operation":                        {},
	"Acquisition failed":                     {},
	"Click Install {content}":                {},
	"Writable":                               {},
	"No write permission":                    {},
	"already installed":                      {},
	"Not installed":                          {},
	"Mysqli extension for PHP is required":   {},
	"File has no write permission:{content}": {},
	"The system has completed installation. If you need to reinstall, please delete the {content} file first": {},
	"Database connection failed:{content}":                        {},
	"Failed to install SQL execution:{content}":                   {},
	"Database does not exist":                                     {},
	"No built front-end file found, please rebuild manually!":     {},
	"Failed to move the front-end file, please move it manually!": {},
	"How to solve?":                       {},
	"View reason":                         {},
	"Click to view the reason":            {},
	"PDO extensions need to be installed": {},
	"proc_open or proc_close functions in PHP Ini is disabled": {},
	"How to modify":               {},
	"Click to view how to modify": {},
	"Security assurance?":         {},
	"Using the installation service correctly will not cause any potential security problems. Click to view the details": {},
	"Please install NPM first":                                                {},
	"Installation error:{content}":                                            {},
	"Please upgrade {content} version":                                        {},
	"The gd extension and freeType library need to be installed":              {},
	"Registration parameter error":                                            {},
	"Check in failed, please try again or contact the website administrator~": {},
	"Member center disabled":                                                  {},
	"Unknown operation":                                                       {},
	"No action available, please contact the administrator~":                  {},
	"Email is exist!":                                                         {},
	// 以下三条由 admin_rule.remark 经 handler/remark.go 的 remark() 动态查找
	// （util.Lang），从 PHP 上游迁移来的库备注仍保存这些英文键；源码 grep
	// 无法证明其死链，与 Dashboard remark / Attachment Remark 同类保留。
	"Role remark":          {},
	"DataRecycle Remark":   {},
	"SensitiveData Remark": {},
}

// 下游按“各语言键集与 en 完全一致”维护语言包；框架侧先守住该不变量，
// 避免某一语言包漏删/漏加键后下游只能自行维护差集清单。
func TestLocalePacksShareKeySet(t *testing.T) {
	t.Parallel()
	packs := loadLocalePacks(t)
	en, ok := packs["en"]
	require.True(t, ok, "en.yaml 缺失")
	require.NotEmpty(t, en)
	for name, keys := range packs {
		require.ElementsMatch(t, mapKeys(en), mapKeys(keys), "语言包 %s 的键集与 en.yaml 不一致", name)
	}
}

// en.yaml 的每条消息都必须能在后端 Go 代码、迁移种子或配置数据中命中，
// 防止不可达消息再次沉淀成下游的对齐噪音。白名单只接受
// legacyUnreferencedKeys 中列明的历史消息，新增键必须在使用处引用。
func TestLocaleKeysAreReferenced(t *testing.T) {
	t.Parallel()
	packs := loadLocalePacks(t)
	en, ok := packs["en"]
	require.True(t, ok, "en.yaml 缺失")

	sources := readReferenceSources(t)
	var unreferenced []string
	for key := range en {
		if strings.Contains(sources, key) {
			continue
		}
		if _, retained := legacyUnreferencedKeys[key]; retained {
			continue
		}
		unreferenced = append(unreferenced, key)
	}
	sort.Strings(unreferenced)
	require.Empty(t, unreferenced,
		"en.yaml 存在无引用的死键，请删除或在 legacyUnreferencedKeys 中写明保留理由：%v", unreferenced)
}

// codeMessageWhitelist 是框架代码抛出、但有意不写进语言包的消息。默认必须
// 为空：新增消息要么整句作为 en.yaml 的键（带值的消息改用 util.Lang 插值），
// 要么在此写明保留理由。由变量拼装的消息 guard 无法判定，直接跳过。
var codeMessageWhitelist = map[string]struct{}{}

// TestCodeMessagesHaveLocaleKeys 守住反向契约：框架代码直接抛给客户端的消息、
// 或作为语言包键交给查表的消息字面量，必须是 en.yaml 的键。覆盖：
//   - 错误包快捷构造（BadRequest / Unauthorized / ForbiddenRequest / InternalServer / New）；
//   - util.Lang 的 messageID；
//   - response.Fail / JsonReturn / SuccessWithMessage；
//   - validator.ValidatorMessages 里每条自定义文案（校验失败会原样回给客户端）；
//   - requesttx.Outcome 的 Message 字段。
//
// 包别名按 import 路径解析，第三方同名包（util/response/error）不会误报，业务
// 用任意别名也照样命中。空消息视为「无文案」跳过。
//
// 不覆盖：response.WriteDirect（有意跳过本地化，调用方必须自行查表）；
// 变量/常量拼装的消息（无法静态判定）；fmt.Errorf（多为带上下文的内部错误，
// 由 errorOutcome 原样暴露，是否可翻译不在本守卫承诺内）。
func TestCodeMessagesHaveLocaleKeys(t *testing.T) {
	t.Parallel()
	en, ok := loadLocalePacks(t)["en"]
	require.True(t, ok, "en.yaml 缺失")

	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	var missing, fragments []string
	require.NoError(t, filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "web", "public", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		// 测试文件里的字面量是断言期望值，不是框架抛出的消息。
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("解析 %s: %w", path, err)
		}
		for _, site := range messageSites(file, fset) {
			rel, relErr := filepath.Rel(root, site.position.Filename)
			require.NoError(t, relErr)
			label := fmt.Sprintf("%s:%d %s", rel, site.position.Line, site.kind)
			if site.fragment {
				fragments = append(fragments, label)
				continue
			}
			if _, exists := en[site.value]; exists {
				continue
			}
			if _, retained := codeMessageWhitelist[site.value]; retained {
				continue
			}
			missing = append(missing, fmt.Sprintf("%s %q", label, site.value))
		}
		return nil
	}))
	sort.Strings(fragments)
	sort.Strings(missing)
	require.Empty(t, fragments,
		"消息由字符串拼接或 fmt.Sprintf 构造，无法作为 en.yaml 的静态键翻译；请改成完整句子（值走 util.Lang 插值）：%v", fragments)
	require.Empty(t, missing,
		"代码抛出的消息在 en.yaml 中没有键，非英语请求会回退英文原文；请补键或在 codeMessageWhitelist 写明保留理由：%v", missing)
}

const (
	errPkgPath       = "buildadmin-go/internal/pkg/error"
	utilPkgPath      = "buildadmin-go/internal/pkg/util"
	responsePkgPath  = "buildadmin-go/internal/pkg/response"
	validatorPkgPath = "buildadmin-go/internal/pkg/validator"
	requesttxPkgPath = "buildadmin-go/internal/pkg/requesttx"
)

// messageSite 是一处需要语言包键的消息字面量（或无法翻译的拼接片段）。
type messageSite struct {
	kind     string
	value    string
	fragment bool
	position token.Position
}

func messageSites(file *ast.File, fset *token.FileSet) []messageSite {
	imports := importAliases(file)
	sites := make([]messageSite, 0, 8)
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.CallExpr:
			if kind, argIndex, ok := messageArgument(n, imports); ok && argIndex < len(n.Args) {
				sites = appendMessageSite(sites, kind, n.Args[argIndex], n.Pos(), fset)
			}
		case *ast.CompositeLit:
			sites = appendCompositeSites(sites, n, imports, fset)
		}
		return true
	})
	return sites
}

// appendMessageSite 记录一处消息字面量；空串与无法静态判定的变量不算消息。
func appendMessageSite(sites []messageSite, kind string, expr ast.Expr, pos token.Pos, fset *token.FileSet) []messageSite {
	value, literal, fragment := messageLiteral(expr)
	switch {
	case fragment:
		return append(sites, messageSite{kind: kind, fragment: true, position: fset.Position(pos)})
	case literal && value != "":
		return append(sites, messageSite{kind: kind, value: value, position: fset.Position(pos)})
	}
	return sites
}

// appendCompositeSites 收集复合字面量里的用户文案：validator 自定义消息与
// requesttx 暂存响应消息。
func appendCompositeSites(sites []messageSite, lit *ast.CompositeLit, imports map[string]string, fset *token.FileSet) []messageSite {
	switch compositeKind(lit, imports) {
	case "validator.ValidatorMessages":
		for _, elt := range lit.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				sites = appendMessageSite(sites, "validator.ValidatorMessages", kv.Value, kv.Pos(), fset)
			}
		}
	case "requesttx.Outcome":
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Message" {
				sites = appendMessageSite(sites, "requesttx.Outcome.Message", kv.Value, kv.Pos(), fset)
			}
		}
	}
	return sites
}

// importAliases 返回文件内「本地别名 → import 路径」映射；按路径而不是标识符
// 名匹配，避免第三方 util/response/error 误报，也支持业务自定义别名。
func importAliases(file *ast.File) map[string]string {
	aliases := make(map[string]string, len(file.Imports))
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(importPath)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "_" || name == "." {
			continue
		}
		aliases[name] = importPath
	}
	return aliases
}

// messageArgument 识别需要语言包的出参消息调用，返回调用名与消息参数下标。
func messageArgument(call *ast.CallExpr, imports map[string]string) (string, int, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", 0, false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", 0, false
	}
	pkgPath, ok := imports[pkg.Name]
	if !ok {
		return "", 0, false
	}
	kind := path.Base(pkgPath) + "." + sel.Sel.Name
	switch pkgPath {
	case errPkgPath:
		switch sel.Sel.Name {
		case "BadRequest", "Unauthorized", "ForbiddenRequest", "InternalServer":
			return kind, 0, true
		case "New":
			return kind, 2, true
		}
	case utilPkgPath:
		if sel.Sel.Name == "Lang" {
			return kind, 1, true
		}
	case responsePkgPath:
		switch sel.Sel.Name {
		case "Fail", "JsonReturn":
			return kind, 3, true
		case "SuccessWithMessage":
			return kind, 1, true
		}
	}
	return "", 0, false
}

// compositeKind 识别携带用户文案的复合字面量类型。
func compositeKind(lit *ast.CompositeLit, imports map[string]string) string {
	sel, ok := lit.Type.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	switch imports[pkg.Name] {
	case validatorPkgPath:
		if sel.Sel.Name == "ValidatorMessages" {
			return "validator.ValidatorMessages"
		}
	case requesttxPkgPath:
		if sel.Sel.Name == "Outcome" {
			return "requesttx.Outcome"
		}
	}
	return ""
}

// messageLiteral 判定消息参数形态：整串字面量、拼接/格式化片段，或无从判断
// 的变量与常量（后两者不参与校验）。
func messageLiteral(expr ast.Expr) (value string, literal bool, fragment bool) {
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false, false
		}
		unquoted, err := strconv.Unquote(v.Value)
		if err != nil {
			return v.Value, true, false
		}
		return unquoted, true, false
	case *ast.BinaryExpr:
		return "", false, leadingStringLiteral(v)
	case *ast.CallExpr:
		return "", false, isSprintfWithLiteralFormat(v)
	case *ast.ParenExpr:
		return messageLiteral(v.X)
	}
	return "", false, false
}

// leadingStringLiteral 判断拼接表达式是否以字面量开头：「字面量 + 变量」最容易被
// 误当成 en.yaml 的键，但查表必然落空，必须改成完整句子或改用 util.Lang 插值。
// 变量在前的动态拼接（如字段名 + " is exist"）没有可查表的静态文本，跳过。
func leadingStringLiteral(expr ast.Expr) bool {
	switch v := expr.(type) {
	case *ast.BasicLit:
		return v.Kind == token.STRING
	case *ast.BinaryExpr:
		return leadingStringLiteral(v.X)
	case *ast.ParenExpr:
		return leadingStringLiteral(v.X)
	}
	return false
}

func isSprintfWithLiteralFormat(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Sprintf" || len(call.Args) == 0 {
		return false
	}
	if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "fmt" {
		return false
	}
	format, ok := call.Args[0].(*ast.BasicLit)
	return ok && format.Kind == token.STRING
}

func loadLocalePacks(t *testing.T) map[string]map[string]string {
	t.Helper()
	entries, err := fs.ReadDir(localesFS, "locales")
	require.NoError(t, err)
	packs := make(map[string]map[string]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		content, err := fs.ReadFile(localesFS, filepath.ToSlash(filepath.Join("locales", entry.Name())))
		require.NoError(t, err)
		var pack map[string]string
		require.NoError(t, yaml.Unmarshal(content, &pack), "解析语言包 %s", entry.Name())
		name := strings.TrimSuffix(entry.Name(), ".yaml")
		packs[name] = pack
	}
	return packs
}

func mapKeys(pack map[string]string) []string {
	keys := make([]string, 0, len(pack))
	for key := range pack {
		keys = append(keys, key)
	}
	return keys
}

// readReferenceSources 拼接后端源码、迁移种子与配置/spec 数据；
// 语言包目录与前端目录不参与匹配，避免键在 en.yaml 里自证引用。
func readReferenceSources(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "..")
	self := filepath.Join(root, "internal", "i18n", "locales_test.go")
	skipDirs := map[string]struct{}{
		filepath.Join(root, ".git"):         {},
		filepath.Join(root, "web"):          {},
		filepath.Join(root, "public"):       {},
		filepath.Join(root, "node_modules"): {},
	}
	locales := filepath.Join(root, "internal", "i18n", "locales")
	var builder strings.Builder
	require.NoError(t, filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path == locales {
				return filepath.SkipDir
			}
			if _, skip := skipDirs[path]; skip {
				return filepath.SkipDir
			}
			return nil
		}
		// 本测试自身携带白名单字面量，不参与引用匹配，否则白名单会自证。
		if path == self {
			return nil
		}
		switch filepath.Ext(path) {
		case ".go", ".yaml", ".yml":
		default:
			return nil
		}
		content, err := os.ReadFile(path) //nolint:gosec // G304: path 来自 WalkDir 遍历仓库目录，仅用于测试期引用扫描
		if err != nil {
			return err
		}
		builder.Write(content)
		builder.WriteByte('\n')
		return nil
	}), "扫描仓库引用来源")
	return builder.String()
}
