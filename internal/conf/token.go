package conf

type Token struct {
	Default string `mapstructure:"default" json:"default" yaml:"default"`
	Key     string `mapstructure:"key" json:"key" yaml:"key"`
	Algo    string `mapstructure:"algo" json:"algo" yaml:"algo"`
}
