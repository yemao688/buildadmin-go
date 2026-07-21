package model

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/random"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"html"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UploadHelper struct {
	config     *conf.Configuration
	sqlDB      *gorm.DB
	oss        *AliossStorage
	file       *multipart.FileHeader
	topic      string //细目（存储目录）
	sourceType string
}

type OSSCallback struct {
	URL    string `json:"url" form:"url" binding:"required"`
	Name   string `json:"name" form:"name"`
	Size   int32  `json:"size" form:"size"`
	Type   string `json:"type" form:"type"`
	Sha1   string `json:"sha1" form:"sha1"`
	Topic  string `json:"topic" form:"topic"`
	Width  int32  `json:"width" form:"width"`
	Height int32  `json:"height" form:"height"`
}

var stripHTMLTags = regexp.MustCompile(`<[^>]*>`)

func normalizeCallbackURL(value string) string {
	return "/" + strings.TrimLeft(strings.ReplaceAll(value, "\\", "/"), "/")
}

func cleanUploadName(name string) string {
	name = html.EscapeString(stripHTMLTags.ReplaceAllString(name, ""))
	runes := []rune(name)
	if len(runes) > 100 {
		runes = runes[:100]
	}
	return string(runes)
}

func (s *UploadHelper) CompleteOSS(params OSSCallback, adminID, userID int32) (Attachment, error) {
	if params.Topic == "" {
		params.Topic = "default"
	}
	attachment := Attachment{Topic: params.Topic, AdminID: adminID, UserID: userID, URL: normalizeCallbackURL(params.URL), Name: cleanUploadName(params.Name), Size: params.Size, Mimetype: params.Type, Width: params.Width, Height: params.Height, Storage: "alioss", Sha1: params.Sha1, Quote: 1, LastUploadTime: time.Now().Unix()}
	if attachment.Sha1 != "" {
		var old Attachment
		if err := s.sqlDB.Where("sha1=? and topic=? and storage=?", attachment.Sha1, attachment.Topic, "alioss").Take(&old).Error; err == nil {
			if s.oss.Exists(old.URL) {
				s.sqlDB.Model(&old).Updates(map[string]any{"quote": old.Quote + 1, "last_upload_time": time.Now().Unix()})
				old.FullUrl = s.oss.URL(old.URL)
				return old, nil
			}
			s.sqlDB.Delete(&old)
		}
	}
	if err := s.sqlDB.Create(&attachment).Error; err != nil {
		return Attachment{}, err
	}
	attachment.FullUrl = s.oss.URL(attachment.URL)
	return attachment, nil
}

func NewUploadHelper(sqlDB *gorm.DB, config *conf.Configuration, ossStorage *AliossStorage) *UploadHelper {
	return &UploadHelper{
		config: config,
		sqlDB:  sqlDB,
		oss:    ossStorage,
		topic:  "default",
	}
}

func (s *UploadHelper) SetFile(file *multipart.FileHeader) map[string]any {
	s.file = file
	s.sourceType = s.file.Header.Get("Content-Type")

	fileInfo := map[string]any{}
	suffix := s.getSuffix()
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
	suffix := s.getSuffix()
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
		msg := utils.Lang(ctx, "The uploaded file is too large (%sMiB), Maximum file size:%sMiB", map[string]string{
			"min": fmt.Sprintf("%d", s.file.Size),
			"max": fmt.Sprintf("%d", s.config.Upload.Maxsize),
		})
		return cErr.BadRequest(msg, 10002)
	}
	return nil
}

func (s *UploadHelper) uploadMode() string {
	if s.oss != nil {
		if c, err := s.oss.settings(); err == nil && c.Mode != "" {
			return c.Mode
		}
	}
	return s.config.Upload.Mode
}

// 获取文件后缀
func (s *UploadHelper) getSuffix() string {
	suffix := strings.TrimLeft(filepath.Ext(s.file.Filename), ".")
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
		dotSuffix = "." + suffix
	}

	replaceArr := map[string]string{
		"{topic}":    s.topic,
		"{year}":     fmt.Sprintf("%04d", now.Year()),
		"{mon}":      fmt.Sprintf("%02d", now.Month()),
		"{day}":      fmt.Sprintf("%02d", now.Day()),
		"{hour}":     fmt.Sprintf("%02d", now.Hour()),
		"{min}":      fmt.Sprintf("%02d", now.Minute()),
		"{sec}":      fmt.Sprintf("%02d", now.Second()),
		"{random}":   random.Build("alnum", 8),
		"{random32}": random.Build("alnum", 32),
		"{filename}": filename,
		"{suffix}":   suffix,
		"{.suffix}":  dotSuffix,
		"{filesha1}": sha1,
	}
	saveName := s.config.Upload.Savename
	for k, v := range replaceArr {
		saveName = strings.Replace(saveName, k, v, 1)
	}

	return saveName
}

func (s *UploadHelper) Upload(ctx *gin.Context, adminId int32, userId int32) (any, error) {
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

	var buffer bytes.Buffer
	_, err = io.Copy(&buffer, fileReader)
	if err != nil {
		return nil, err
	}

	//获取文件sha1值
	hasher := sha1.New()
	if _, err := io.Copy(hasher, bytes.NewReader(buffer.Bytes())); err != nil {
		return nil, err
	}
	fileSHA1 := hasher.Sum(nil)
	sha1String := fmt.Sprintf("%x", fileSHA1)
	savePath := s.getSaveName(sha1String)
	//如果是图片,计算图片宽高
	isImage := s.checkIsImage()
	width := 0
	height := 0
	if isImage {
		img, _, err := image.Decode(bytes.NewReader(buffer.Bytes()))
		if err != nil {
			return nil, err
		}
		width, height = img.Bounds().Dx(), img.Bounds().Dy()
	}

	attach := Attachment{}
	storage := "local"
	if s.uploadMode() == "alioss" {
		storage = "alioss"
	}
	if err := s.sqlDB.Where("sha1=? and topic=? and storage=?", sha1String, s.topic, storage).Take(&attach).Error; err == nil {
		//判断文件是否存在
		missing := attach.Storage == "local" && !utils.PathExists(utils.RootPath()+attach.URL)
		if attach.Storage == "alioss" && s.oss != nil {
			missing = !s.oss.Exists(attach.URL)
		}
		if missing {
			s.sqlDB.Model(&Attachment{}).Where("id=?", attach.ID).Delete(nil)
		} else {
			s.sqlDB.Model(&Attachment{}).Where("id=?", attach.ID).Updates(map[string]any{
				"quote":            attach.Quote + 1,
				"last_upload_time": time.Now().Unix(),
			})
			attach.Suffix = strings.TrimLeft(filepath.Ext(attach.URL), ".")
			if storage == "alioss" && s.oss != nil {
				attach.FullUrl = s.oss.URL(attach.URL)
			} else {
				attach.FullUrl = utils.FullUrl(attach.URL, s.config.App.CdnUrl, utils.GetBaseURL(ctx), "")
			}
			return attach, nil
		}
	}

	attachment := Attachment{
		Topic:          s.topic,
		AdminID:        adminId,
		UserID:         userId,
		URL:            savePath,
		Width:          int32(width),
		Height:         int32(height),
		Name:           s.file.Filename,
		Size:           int32(s.file.Size),
		Mimetype:       s.sourceType,
		Storage:        storage,
		Sha1:           sha1String,
		Quote:          1,
		LastUploadTime: time.Now().Unix(),
	}
	if err := s.sqlDB.Create(&attachment).Error; err != nil {
		return nil, err
	}
	attachment.Suffix = s.getSuffix()
	if storage == "alioss" && s.oss != nil {
		if err := s.oss.Save(bytes.NewReader(buffer.Bytes()), savePath); err != nil {
			s.sqlDB.Delete(&attachment)
			return nil, err
		}
		attachment.FullUrl = s.oss.URL(savePath)
		return attachment, nil
	}
	attachment.FullUrl = utils.FullUrl(savePath, s.config.App.CdnUrl, utils.GetBaseURL(ctx), "")

	dirPath := filepath.Dir(utils.RootPath() + savePath)
	// 尝试创建路径中所有不存在的目录
	err = os.MkdirAll(dirPath, 0755)
	if err != nil {
		return nil, err
	}
	// 创建目标文件
	out, err := os.Create(utils.RootPath() + savePath)
	if err != nil {
		return nil, err
	}
	defer out.Close()

	// 将上传的文件内容写入到目标文件
	_, err = io.Copy(out, bytes.NewReader(buffer.Bytes()))
	if err != nil {
		return nil, err
	}
	return attachment, nil
}
