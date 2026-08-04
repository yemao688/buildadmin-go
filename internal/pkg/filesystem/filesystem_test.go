package filesystem

import "testing"

func TestFileUnitToByte(t *testing.T) {
	list := []struct {
		data     string
		expected int64
	}{
		{"5mb", 5242880},
		{"2Gb", 2147483648},
	}

	// 遍历测试用例并执行测试
	for _, v := range list {
		want, _ := FileUnitToByte(v.data)
		if want != v.expected {
			t.Errorf("result %d; want %d", want, v.expected)
		}
	}
}
