package depends

import (
	"encoding/json"
	cErr "go-build-admin/app/pkg/error"
	"os"
)

type Depend struct {
	JsonContent map[string]any
	J           string
	T           string
}

func (d *Depend) NewDepend(j string, t string) *Depend {
	return &Depend{
		JsonContent: map[string]any{},
		J:           j,
		T:           t,
	}
}

// 获取 json 文件内容
func (d *Depend) GetContent(realTime bool) (map[string]any, error) {
	_, err := os.Stat(d.J)
	if err != nil {
		return nil, cErr.BadRequest(d.J + " file does not exist!")
	}

	if len(d.JsonContent) > 0 && !realTime {
		return d.JsonContent, nil
	}

	content, _ := os.ReadFile(d.J)
	data := map[string]any{}
	err = json.Unmarshal([]byte(content), &data)
	return data, err
}

// 设置 json 文件内容
func (d *Depend) SetContent(content map[string]any) error {
	if len(content) == 0 {
		content = d.JsonContent
	}

	if _, ok := content["name"]; !ok {
		return cErr.BadRequest("Depend content file content is incomplete")
	}

	value, err := json.Marshal(content)
	if err != nil {
		return err
	}
	return os.WriteFile(d.J, []byte(value), 0644)
}

// 获取依赖项
func (d *Depend) GetDepends(devEnv bool) (any, error) {
	content, err := d.GetContent(false)
	if err != nil {
		return nil, err
	}

	if d.T == "npm" {
		if devEnv {
			return content["devDependencies"], nil
		}
		return content["dependencies"], nil
	}

	if devEnv {
		return content["require-dev"], nil
	}
	return content["require"], nil
}

// 是否存在某个依赖
func (d *Depend) HasDepend(name string, devEnv bool) (string, error) {
	_, err := d.GetDepends(devEnv)
	if err != nil {
		return "", err
	}
	// if v, ok := depends[name]; ok {
	// 	return v, err
	// }
	return "", nil
}

// 添加依赖
func (d *Depend) AddDepends() {

}

// 删除依赖
func (d *Depend) RemoveDepends() {

}
