package conf

type App struct {
	Env      string `mapstructure:"env" json:"env" yaml:"env"`
	Port     string `mapstructure:"port" json:"port" yaml:"port"`
	AppName  string `mapstructure:"app_name" json:"app_name" yaml:"app_name"`
	AppUrl   string `mapstructure:"app_url" json:"app_url" yaml:"app_url"`
	TimeZone string `mapstructure:"time_zone" json:"time_zone" yaml:"time_zone"`
}
