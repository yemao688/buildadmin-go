package i18n

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
