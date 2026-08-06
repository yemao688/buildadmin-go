package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
)

// 递归删除目录
func DelDir(dir string) error {
	// 检查路径是否存在且是目录
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("路径不是一个目录")
	}
	err = os.RemoveAll(dir)
	if err != nil {
		return err
	}
	return nil
}

// 获取一个目录内的文件列表
func GetDirFiles(dirPath string, suffixArr []string) map[string]string {
	result := map[string]string{}
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			if len(suffixArr) != 0 {
				filePath := filepath.Join(dirPath, entry.Name())
				ext := filepath.Ext(filePath)
				if slices.Contains(suffixArr, ext) {
					result[entry.Name()] = entry.Name()
				}
			} else {
				result[entry.Name()] = entry.Name()
			}
		}
	}
	return result
}
