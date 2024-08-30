package filesystem

import (
	"archive/zip"
	"errors"
	"fmt"
	"go-build-admin/utils"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// 是否是空目录
func DirIsEmpty(dir string) bool {
	// 检查目录是否存在
	stat, err := os.Stat(dir)
	if err != nil || !stat.IsDir() {
		return false
	}

	// 读取目录内容
	files, err := os.ReadDir(dir)
	if err != nil {
		return false // 如果读取目录出错，默认认为非空（避免误判）
	}

	// 检查目录是否为空
	for _, file := range files {
		// 忽略"."和".."这两个特殊目录
		if file.Name() != "." && file.Name() != ".." {
			return false
		}
	}
	return true
}

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

// 删除一个路径下的所有相对空文件夹（删除此路径中的所有空文件夹）
func DelEmptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	// 遍历目录下的所有条目
	for _, entry := range entries {
		entryPath := filepath.Join(dir, entry.Name())
		// 如果是目录且不为"."或".."
		if entry.IsDir() && entry.Name() != "." && entry.Name() != ".." {
			// 递归检查并尝试删除子目录
			if err := DelEmptyDir(entryPath); err != nil {
				return err
			}
		}
	}

	if DirIsEmpty(dir) {
		err := os.Remove(dir)
		if err != nil {
			return err
		}
	}
	return nil
}

// 检查目录/文件是否可写
func PathIsWritable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// 检查文件权限，ModePerm提供所有权限位
	if info.Mode().Perm()&0200 != 0 {
		// 0200 对应于用户写权限，如果文件或目录至少对用户可写，则认为是可写的
		return true
	}
	return false
}

// 解压Zip
func Unzip(src string, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		// 创建目录结构
		path := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			err = os.MkdirAll(path, f.Mode())
			if err != nil {
				return err
			}
			continue
		}

		// 创建文件
		if err = os.MkdirAll(filepath.Dir(path), f.Mode()); err != nil {
			return err
		}
		w, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		defer w.Close()

		// 复制内容
		_, err = io.Copy(w, rc)
		if err != nil {
			return err
		}
	}
	return nil
}

// 创建ZIP
func Zip(files []string, zipfileName string, erasePre string) error {
	zipFile, err := os.Create(filepath.Join(utils.RootPath(), zipfileName))
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	rootPath := utils.RootPath()
	for _, filePath := range files {
		// 处理路径
		fullFilePath := filepath.Join(rootPath, filePath)
		// 检查文件是否存在
		if fileInfo, err := os.Stat(fullFilePath); os.IsNotExist(err) {
			return fmt.Errorf("文件不存在: %s", fullFilePath)
		} else if fileInfo.IsDir() {
			// 目录处理逻辑，此处简化处理，仅作示例
			return fmt.Errorf("目录暂未处理: %s", fullFilePath)
		}

		// 添加文件到ZIP
		zipEntry, err := zipWriter.Create(strings.ReplaceAll(filePath, erasePre, ""))
		if err != nil {
			return err
		}
		fmt.Println(fullFilePath)
		file, err := os.Open(fullFilePath)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(zipEntry, file)
		if err != nil {
			return err
		}
	}
	return nil
}

// 递归创建目录
func Mkdir(dir string) bool {
	err := os.MkdirAll(dir, 0755)
	if err != nil && !os.IsExist(err) {
		return false
	}
	return true
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

// 将一个文件单位转为字节
func FileUnitToByte(sizeStr string) (int64, error) {
	re := regexp.MustCompile(`([0-9.]+)(\w+)`)
	parts := re.FindStringSubmatch(sizeStr)
	if parts == nil {
		return 0, nil
	}

	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	size, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %s", parts[0])
	}

	unit := strings.ToLower(parts[2])
	unitMultipliers := map[string]int64{
		"b":  1,
		"k":  1 << 10,
		"kb": 1 << 10,
		"m":  1 << 20,
		"mb": 1 << 20,
		"g":  1 << 30,
		"gb": 1 << 30,
	}

	multiplier, exists := unitMultipliers[unit]
	if !exists {
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
	return size * multiplier, nil
}
