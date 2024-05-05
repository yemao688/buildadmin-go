package conf

type Configuration struct {
	App          App          `mapstructure:"app" json:"app" yaml:"app"`
	Log          Log          `mapstructure:"log" json:"log" yaml:"log"`
	Database     Database     `mapstructure:"mysql" json:"mysql" yaml:"mysql"`
	Jwt          Jwt          `mapstructure:"jwt" json:"jwt" yaml:"jwt"`
	Redis        Redis        `mapstructure:"redis" json:"redis" yaml:"redis"`
	Token        Token        `mapstructure:"token" json:"token" yaml:"token"`
	Terminal     Terminal     `mapstructure:"terminal" json:"terminal" yaml:"terminal"`
	ClickCaptcha ClickCaptcha `mapstructure:"click_captcha" json:"click_captcha" yaml:"click_captcha"`
}
