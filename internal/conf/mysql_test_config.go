package conf

// MysqlTest 是 MySQL 集成测试的专用连接配置（configs/config.yaml 的 mysql_test 段）。
// 开发机需自建一次性测试库并对账号授权；未配置或 enabled=false 时相关测试跳过。
type MysqlTest struct {
	Enabled  bool   `mapstructure:"enabled" json:"enabled" yaml:"enabled"`
	Host     string `mapstructure:"host" json:"host" yaml:"host"`
	Port     int    `mapstructure:"port" json:"port" yaml:"port"`
	Database string `mapstructure:"database" json:"database" yaml:"database"`
	UserName string `mapstructure:"username" json:"username" yaml:"username"`
	Password string `mapstructure:"password" json:"password" yaml:"password"`
	Charset  string `mapstructure:"charset" json:"charset" yaml:"charset"`
}
