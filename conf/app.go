package conf

type App struct {
	Env                string `mapstructure:"env" json:"env" yaml:"env"`
	Port               string `mapstructure:"port" json:"port" yaml:"port"`
	AppName            string `mapstructure:"app_name" json:"app_name" yaml:"app_name"`
	AppUrl             string `mapstructure:"app_url" json:"app_url" yaml:"app_url"`
	CorsRequestDomain  string `mapstructure:"cors_request_domain" json:"cors_request_domain" yaml:"cors_request_domain"`
	AdminLoginCaptcha  bool   `mapstructure:"admin_login_captcha" json:"admin_login_captcha" yaml:"admin_login_captcha"`
	UserLoginRetry     int    `mapstructure:"user_login_retry" json:"user_login_retry" yaml:"user_login_retry"`
	AdminLoginRetry    int    `mapstructure:"admin_login_retry" json:"admin_login_retry" yaml:"admin_login_retry"`
	AdminSso           bool   `mapstructure:"admin_sso" json:"admin_sso" yaml:"admin_sso"`
	UserSso            bool   `mapstructure:"user_sso" json:"user_sso" yaml:"user_sso"`
	UserTokenKeepTime  int    `mapstructure:"user_token_keep_time" json:"user_token_keep_time" yaml:"user_token_keep_time"`
	AdminTokenKeepTime int    `mapstructure:"admin_token_keep_time" json:"admin_token_keep_time" yaml:"admin_token_keep_time"`
	AutoSortEqWeight   bool   `mapstructure:"auto_sort_eq_weight" json:"auto_sort_eq_weight" yaml:"auto_sort_eq_weight"`
	OpenMemberCenter   bool   `mapstructure:"open_member_center" json:"open_member_center" yaml:"open_member_center"`
	ModulePureInstall  bool   `mapstructure:"module_pure_install" json:"module_pure_install" yaml:"module_pure_install"`
	AutoWriteAdminLog  bool   `mapstructure:"auto_write_admin_log" json:"auto_write_admin_log" yaml:"auto_write_admin_log"`
	DefaultAvatar      string `mapstructure:"default_avatar" json:"default_avatar" yaml:"default_avatar"`
	CdnUrl             string `mapstructure:"cdn_url" json:"cdn_url" yaml:"cdn_url"`
	Version            string `mapstructure:"version" json:"version" yaml:"version"`
	ApiUrl             string `mapstructure:"api_url" json:"api_url" yaml:"api_url"`
}
