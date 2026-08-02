package dto

type LanguageContentParam struct {
	Lan   string `json:"lan"`   // 语言代码
	Group string `json:"group"` // 分组
	Key   string `json:"key"`   // 键
	Type  string `json:"type"`  // 类型:0=文本,1=富文本,2=图片
	Value string `json:"value"` // 值
}
