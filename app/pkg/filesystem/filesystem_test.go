package filesystem

import (
	"fmt"
	"go-build-admin/utils"
	"path/filepath"
	"testing"
)

func TestDirIsEmpty(t *testing.T) {
	dirPath := filepath.Join(utils.RootPath(), "app/pkg/filesystem/test")
	fmt.Println(DirIsEmpty(dirPath))
}

func TestZip(t *testing.T) {

}

func TestMkdir(t *testing.T) {
	dirPath := filepath.Join(utils.RootPath(), "app/pkg/filesystem/test/aa/bb")
	Mkdir(dirPath)
}

func TestGetDirFiles(t *testing.T) {
	dirPath := utils.RootPath()
	suffixArr := []string{".sum"}
	result := GetDirFiles(dirPath, suffixArr)
	fmt.Println(result)
}

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
