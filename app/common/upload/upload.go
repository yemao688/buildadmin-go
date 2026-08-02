package upload

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

// UploadHelper 是无状态共享服务：实例由 Wire 以单例注入，在并发请求间共享。
// 请求级状态（文件、细目）一律通过 UploadParams 逐调用传入，禁止挂载到实例上。
type UploadHelper struct {
	config *conf.Configuration
	sqlDB  *gorm.DB
	oss    *AliossStorage
}

// UploadParams 单次上传请求的输入值。
type UploadParams struct {
	File  *multipart.FileHeader
	Topic string //细目（存储目录），空则按 default 处理
}

func (p UploadParams) topic() string {
	if p.Topic == "" {
		return "default"
	}
	return p.Topic
}

func (p UploadParams) sourceType() string {
	return p.File.Header.Get("Content-Type")
}

// 获取文件后缀
func (p UploadParams) suffix() string {
	suffix := strings.TrimLeft(filepath.Ext(p.File.Filename), ".")
	if suffix == "" {
		suffix = "file"
	}
	return suffix
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
				if err := s.sqlDB.Model(&old).Updates(map[string]any{"quote": old.Quote + 1, "last_upload_time": time.Now().Unix()}).Error; err != nil {
					return Attachment{}, err
				}
				old.FullUrl = s.oss.URL(old.URL)
				return old, nil
			}
			if err := s.sqlDB.Delete(&old).Error; err != nil {
				return Attachment{}, err
			}
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
	}
}

// 检查文件类型是否允许上传
func (s *UploadHelper) checkMimetype(sourceType, suffix string) error {
	mimetypeArr := strings.Split(strings.ToLower(s.config.Upload.Mimetype), ",")
	sourceTypeArr := strings.Split(sourceType, ",")
	// 验证文件后缀
	if s.config.Upload.Mimetype == "*" {
		return nil
	}
	if slices.Contains(mimetypeArr, suffix) {
		return nil
	}

	if slices.Contains(mimetypeArr, "."+suffix) {
		return nil
	}

	if slices.Contains(mimetypeArr, sourceType) {
		return nil
	}

	if slices.Contains(mimetypeArr, sourceTypeArr[0]+"/*") {
		return nil
	}
	return cErr.BadRequest("The uploaded file format is not allowed", 10002)
}

// 是否是图片
func (s *UploadHelper) checkIsImage(sourceType, suffix string) bool {
	typeArr := []string{"image/gif", "image/jpg", "image/jpeg", "image/bmp", "image/png", "image/webp"}
	suffixArr := []string{"gif", "jpg", "jpeg", "bmp", "png", "webp"}
	return slices.Contains(typeArr, sourceType) || slices.Contains(suffixArr, suffix)
}

// 检查文件大小是否允许上传
func (s *UploadHelper) checkSize(ctx *gin.Context, file *multipart.FileHeader) error {
	if file.Size > int64(s.config.Upload.Maxsize) {
		msg := utils.Lang(ctx, "The uploaded file is too large (%sMiB), Maximum file size:%sMiB", map[string]string{
			"min": fmt.Sprintf("%d", file.Size),
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

// 获取文件保存名
func (s *UploadHelper) getSaveName(params UploadParams, sha1 string) string {
	now := time.Now()

	filename := params.File.Filename
	if len(params.File.Filename) > 15 {
		filename = filename[:15]
	}

	suffix := params.suffix()
	dotSuffix := ""
	if suffix != "" {
		dotSuffix = "." + suffix
	}

	replaceArr := map[string]string{
		"{topic}":    params.topic(),
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

func (s *UploadHelper) Upload(ctx *gin.Context, params UploadParams, adminId int32, userId int32) (any, error) {
	if err := s.checkSize(ctx, params.File); err != nil {
		return nil, err
	}
	sourceType := params.sourceType()
	suffix := params.suffix()
	if err := s.checkMimetype(sourceType, suffix); err != nil {
		return nil, err
	}

	fileReader, err := params.File.Open()
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
	savePath := s.getSaveName(params, sha1String)
	//如果是图片,计算图片宽高
	isImage := s.checkIsImage(sourceType, suffix)
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
	if err := s.sqlDB.Where("sha1=? and topic=? and storage=?", sha1String, params.topic(), storage).Take(&attach).Error; err == nil {
		//判断文件是否存在
		missing := attach.Storage == "local" && !utils.PathExists(utils.RootPath()+attach.URL)
		if attach.Storage == "alioss" && s.oss != nil {
			missing = !s.oss.Exists(attach.URL)
		}
		if missing {
			if err := s.sqlDB.Model(&Attachment{}).Where("id=?", attach.ID).Delete(nil).Error; err != nil {
				return nil, err
			}
		} else {
			if err := s.sqlDB.Model(&Attachment{}).Where("id=?", attach.ID).Updates(map[string]any{
				"quote":            attach.Quote + 1,
				"last_upload_time": time.Now().Unix(),
			}).Error; err != nil {
				return nil, err
			}
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
		Topic:          params.topic(),
		AdminID:        adminId,
		UserID:         userId,
		URL:            savePath,
		Width:          int32(width),
		Height:         int32(height),
		Name:           params.File.Filename,
		Size:           int32(params.File.Size),
		Mimetype:       sourceType,
		Storage:        storage,
		Sha1:           sha1String,
		Quote:          1,
		LastUploadTime: time.Now().Unix(),
	}
	if err := s.sqlDB.Create(&attachment).Error; err != nil {
		return nil, err
	}
	attachment.Suffix = suffix
	if storage == "alioss" && s.oss != nil {
		if err := s.oss.Save(bytes.NewReader(buffer.Bytes()), savePath); err != nil {
			s.sqlDB.Delete(&attachment)
			return nil, err
		}
		attachment.FullUrl = s.oss.URL(savePath)
		return attachment, nil
	}
	attachment.FullUrl = utils.FullUrl(savePath, s.config.App.CdnUrl, utils.GetBaseURL(ctx), "")

	dirPath := filepath.Dir(utils.RootPath() + "/public" + savePath)
	// 尝试创建路径中所有不存在的目录
	err = os.MkdirAll(dirPath, 0755)
	if err != nil {
		return nil, err
	}
	// 创建目标文件
	out, err := os.Create(utils.RootPath() + "/public" + savePath)
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
