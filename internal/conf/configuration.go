package conf

type Configuration struct {
	App          App          `mapstructure:"app" json:"app" yaml:"app"`
	Log          Log          `mapstructure:"log" json:"log" yaml:"log"`
	Database     Database     `mapstructure:"mysql" json:"mysql" yaml:"mysql"`
	MysqlTest    MysqlTest    `mapstructure:"mysql_test" json:"mysql_test" yaml:"mysql_test"`
	Redis        Redis        `mapstructure:"redis" json:"redis" yaml:"redis"`
	Token        Token        `mapstructure:"token" json:"token" yaml:"token"`
	Terminal     Terminal     `mapstructure:"terminal" json:"terminal" yaml:"terminal"`
	ClickCaptcha ClickCaptcha `mapstructure:"click_captcha" json:"click_captcha" yaml:"click_captcha"`
	Upload       Upload       `mapstructure:"upload" json:"upload" yaml:"upload"`
	Crud         Crud         `mapstructure:"crud" json:"crud" yaml:"crud"`
}
