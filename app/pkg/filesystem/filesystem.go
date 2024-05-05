package filesystem

//是否是空目录
func DirIsEmpty(dir string) bool {

	return true
}

//递归删除目录
func DelDir(dir string) bool {
	return true
}

//删除一个路径下的所有相对空文件夹（删除此路径中的所有空文件夹）
func DelEmptyDir(dir string) {

}

//检查目录/文件是否可写
func PathIsWritable(dir string) bool {

	return true
}

//路径分隔符根据当前系统分隔符适配
func FsFit(dir string) string {

	return ""
}

//解压Zip
func Unzip(dir string) string {

	return ""
}

//创建ZIP
func Zip(dir string) string {

	return ""
}

//递归创建目录
func Mkdir(dir string) string {

	return ""
}

//获取一个目录内的文件列表
func GetDirFiles(dir string) string {

	return ""
}

//将一个文件单位转为字节
func FileUnitToByte(dir string) string {

	return ""
}
