package conf

type Upload struct {
	Maxsize  string `mapstructure:"maxsize" json:"maxsize" yaml:"maxsize"`
	Savename string `mapstructure:"savename" json:"savename" yaml:"savename"`
	Mimetype string `mapstructure:"mimetype" json:"mimetype" yaml:"mimetype"`
	Mode     string `mapstructure:"mode" json:"mode" yaml:"mode"`
}
