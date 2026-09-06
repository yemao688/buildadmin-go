package conf

// Translate 配置：多语言表单组件"一键翻译"调用的外部翻译服务。
// API 为翻译服务 base URL，translateMulti 接口挂在 {api}translateMulti；
// Timeout 为单次翻译请求超时（秒，<=0 时客户端回退默认 10s）。
type Translate struct {
	API     string `mapstructure:"api" json:"api" yaml:"api"`
	Timeout int    `mapstructure:"timeout" json:"timeout" yaml:"timeout"`
}