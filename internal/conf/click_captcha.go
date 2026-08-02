package conf

type ClickCaptcha struct {
	Mode          string `mapstructure:"mode" json:"mode" yaml:"mode"`
	Length        int    `mapstructure:"length" json:"length" yaml:"length"`
	ConfuseLength int    `mapstructure:"confuse_length" json:"confuse_length" yaml:"confuse_length"`
}
