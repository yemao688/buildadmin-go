package model

import (
	"crypto/sha1"
	"fmt"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/random"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"image"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UploadHelper struct {
	config     *conf.Configuration
	sqlDB      *gorm.DB
	file       *multipart.FileHeader
	topic      string //细目（存储目录）
	sourceType string
}

func NewUploadHelper(sqlDB *gorm.DB, config *conf.Configuration) *UploadHelper {
	return &UploadHelper{
		config: config,
		sqlDB:  sqlDB,
		topic:  "default",
	}
}

func (s *UploadHelper) SetFile(file *multipart.FileHeader) map[string]any {
	s.file = file
	s.sourceType = s.file.Header.Get("Content-Type")

	fileInfo := map[string]any{}
	suffix := filepath.Ext(s.file.Filename)
	if suffix == "" {
		suffix = "file"
	}
	fileInfo["suffix"] = suffix
	fileInfo["type"] = s.sourceType
	fileInfo["size"] = s.file.Size
	fileInfo["name"] = s.file.Filename
	fileInfo["sha1"] = ""
	return fileInfo
}

func (s *UploadHelper) SetTopic(topic string) {
	s.topic = topic
}

// 检查文件类型是否允许上传
func (s *UploadHelper) checkMimetype() error {
	mimetypeArr := strings.Split(strings.ToLower(s.config.Upload.Mimetype), ",")
	sourceTypeArr := strings.Split(s.sourceType, ",")
	// 验证文件后缀
	if s.config.Upload.Mimetype == "*" {
		return nil
	}

	suffix := filepath.Ext(s.file.Filename)
	if slices.Contains(mimetypeArr, suffix) {
		return nil
	}

	if slices.Contains(mimetypeArr, "."+suffix) {
		return nil
	}

	if slices.Contains(mimetypeArr, s.sourceType) {
		return nil
	}

	if slices.Contains(mimetypeArr, sourceTypeArr[0]+"/*") {
		return nil
	}
	return cErr.BadRequest("The uploaded file format is not allowed", 10002)
}

// 是否是图片
func (s *UploadHelper) checkIsImage() bool {
	typeArr := []string{"image/gif", "image/jpg", "image/jpeg", "image/bmp", "image/png", "image/webp"}
	suffixArr := []string{"gif", "jpg", "jpeg", "bmp", "png", "webp"}
	if slices.Contains(typeArr, s.sourceType) || slices.Contains(suffixArr, s.getSuffix()) {
		return true
	}
	return false
}

// 检查文件大小是否允许上传
func (s *UploadHelper) checkSize(ctx *gin.Context) error {
	if s.file.Size > int64(s.config.Upload.Maxsize) {
		msg := utils.Lang(ctx, "The uploaded file is too large (%sMiB), Maximum file size:%sMiB", map[string]any{"min": s.file.Size, "max": s.config.Upload.Maxsize})
		return cErr.BadRequest(msg, 10002)
	}
	return nil
}

// 获取文件后缀
func (s *UploadHelper) getSuffix() string {
	suffix := filepath.Ext(s.file.Filename)
	if suffix == "" {
		suffix = "file"
	}
	return suffix
}

// 获取文件保存名
func (s *UploadHelper) getSaveName(sha1 string) string {
	now := time.Now()

	filename := s.file.Filename
	if len(s.file.Filename) > 15 {
		filename = filename[:15]
	}

	suffix := s.getSuffix()
	dotSuffix := ""
	if suffix != "" {
		dotSuffix = "." + dotSuffix
	}

	replaceArr := map[string]string{
		"{topic}":    s.topic,
		"{year}":     strconv.Itoa(now.Year()),
		"{mon}":      strconv.Itoa(int(now.Month())),
		"{day}":      strconv.Itoa(now.Day()),
		"{hour}":     strconv.Itoa(now.Hour()),
		"{min}":      strconv.Itoa(now.Minute()),
		"{sec}":      strconv.Itoa(now.Second()),
		"{random}":   random.Build("alnum", 8),
		"{random32}": random.Build("alnum", 32),
		"{filename}": filename,
		"{suffix}":   suffix,
		"{.suffix}":  dotSuffix,
		"{filesha1}": sha1,
	}
	saveName := s.config.Upload.Savename
	for k, v := range replaceArr {
		strings.Replace(saveName, k, v, 1)
	}
	return saveName
}

func (s *UploadHelper) Upload(ctx *gin.Context, saveName string, adminId int32, userId int32) (any, error) {
	if err := s.checkSize(ctx); err != nil {
		return nil, err
	}
	if err := s.checkMimetype(); err != nil {
		return nil, err
	}

	fileReader, err := s.file.Open()
	if err != nil {
		return nil, err
	}
	defer fileReader.Close()

	hasher := sha1.New()
	if _, err := io.Copy(hasher, fileReader); err != nil {
		return nil, err
	}

	fileSHA1 := hasher.Sum(nil)
	sha1String := fmt.Sprintf("%x", fileSHA1)
	savePath := s.getSaveName(sha1String)

	isImage := s.checkIsImage()
	width := 0
	height := 0
	if isImage {
		img, _, err := image.Decode(fileReader)
		if err != nil {
			return nil, err
		}
		width, height = img.Bounds().Dx(), img.Bounds().Dy()
	}

	attachment := Attachment{
		Topic:    s.topic,
		AdminID:  adminId,
		UserID:   userId,
		URL:      savePath,
		Width:    int32(width),
		Height:   int32(height),
		Name:     "",
		Size:     int32(s.file.Size),
		Mimetype: s.sourceType,
		Storage:  "local",
		Sha1:     sha1String,
	}
	if err := s.sqlDB.Table(TableNameAttachment).Create(&attachment).Error; err != nil {
		attach := Attachment{}
		if err := s.sqlDB.Table(TableNameAttachment).Where("sha1=? and topic=? and storage=", sha1String, s.topic, "local").Take(&attach).Error; err != nil {
			return nil, err
		}
		return attach, nil
	}

	// 创建目标文件
	out, err := os.Create(savePath)
	if err != nil {
		return nil, err
	}
	defer out.Close()

	// 将上传的文件内容写入到目标文件
	_, err = io.Copy(out, fileReader)
	if err != nil {
		return nil, err
	}
	return attachment, nil
}
