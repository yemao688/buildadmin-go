package i18n

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
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

// TestCodeMessagesHaveLocaleKeys 守住反向契约：框架代码经 cErr 快捷构造、
// util.Lang、response.Fail/JsonReturn/SuccessWithMessage 直接抛出的消息字面量
// 必须是 en.yaml 的键。缺键时非英语请求经 writeResponse 查表落空，只会看到
// 英文原文；下游也无法从 en 差集判断该消息是否需要翻译。
// response.WriteDirect 不参与：它跳过 msg 本地化，字面量补键并不会翻译它。
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
			case ".git", "web", "public", "node_modules":
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
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			kind, argIndex, ok := messageArgument(call)
			if !ok || argIndex >= len(call.Args) {
				return true
			}
			value, literal, fragment := messageLiteral(call.Args[argIndex])
			position := fset.Position(call.Pos())
			rel, relErr := filepath.Rel(root, position.Filename)
			require.NoError(t, relErr)
			site := fmt.Sprintf("%s:%d %s", rel, position.Line, kind)
			switch {
			case fragment:
				fragments = append(fragments, site)
			case literal:
				if _, exists := en[value]; exists {
					return true
				}
				if _, retained := codeMessageWhitelist[value]; retained {
					return true
				}
				missing = append(missing, fmt.Sprintf("%s %q", site, value))
			}
			return true
		})
		return nil
	}))
	sort.Strings(fragments)
	sort.Strings(missing)
	require.Empty(t, fragments,
		"消息由字符串拼接或 fmt.Sprintf 构造，无法作为 en.yaml 的静态键翻译；请改成完整句子（值走 util.Lang 插值）：%v", fragments)
	require.Empty(t, missing,
		"代码抛出的消息在 en.yaml 中没有键，非英语请求会回退英文原文；请补键或在 codeMessageWhitelist 写明保留理由：%v", missing)
}

// messageArgument 识别需要语言包的出参消息调用，返回调用名与消息参数下标。
func messageArgument(call *ast.CallExpr) (string, int, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", 0, false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", 0, false
	}
	name := pkg.Name + "." + sel.Sel.Name
	switch name {
	case "cErr.BadRequest", "cErr.Unauthorized", "cErr.ForbiddenRequest", "cErr.InternalServer":
		return name, 0, true
	case "cErr.New":
		return name, 2, true
	case "util.Lang":
		return name, 1, true
	case "response.Fail", "response.JsonReturn":
		return name, 3, true
	case "response.SuccessWithMessage":
		return name, 1, true
	}
	return "", 0, false
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
