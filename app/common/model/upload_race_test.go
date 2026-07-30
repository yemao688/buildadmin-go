package model

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// 回归测试：UploadHelper 作为 Wire 单例在并发请求间共享，
// 请求级状态（文件、topic）必须逐调用隔离。
// 每个 goroutine 以不同 topic 和文件内容并发上传，
// 返回的附件记录、落盘路径与文件内容必须与各自输入一一对应。
func TestUploadHelperConcurrentUploadsDoNotShareRequestState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "upload_race.db")), &gorm.Config{
		NamingStrategy:                           schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Attachment{}))

	cfg := &conf.Configuration{
		Upload: conf.Upload{
			Maxsize:  10 << 20,
			Savename: "/storage/{topic}/{year}{mon}{day}/{filename}{filesha1}{.suffix}",
			Mimetype: "*",
		},
	}
	helper := NewUploadHelper(db, cfg, nil)

	newFileHeader := func(name string, content []byte) (*multipart.FileHeader, func()) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", name)
		require.NoError(t, err)
		_, err = part.Write(content)
		require.NoError(t, err)
		require.NoError(t, writer.Close())
		req := httptest.NewRequest("POST", "/ajax/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		require.NoError(t, req.ParseMultipartForm(int64(len(content))+4096))
		return req.MultipartForm.File["file"][0], func() { _ = req.MultipartForm.RemoveAll() }
	}

	const workers = 6
	topics := make([]string, workers)
	contents := make([][]byte, workers)
	headers := make([]*multipart.FileHeader, workers)
	cleanups := make([]func(), 0, workers)
	for i := 0; i < workers; i++ {
		topics[i] = fmt.Sprintf("race-%c", 'a'+rune(i))
		contents[i] = []byte(fmt.Sprintf("upload-race-content-%d", i))
		fh, cleanup := newFileHeader(fmt.Sprintf("race-%c.txt", 'a'+rune(i)), contents[i])
		headers[i] = fh
		cleanups = append(cleanups, cleanup)
	}
	defer func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
		for _, topic := range topics {
			_ = os.RemoveAll(filepath.Join(utils.RootPath(), "public", "storage", topic))
		}
	}()

	type outcome struct {
		idx int
		att Attachment
		err error
	}
	results := make(chan outcome, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/ajax/upload", nil)
			got, err := helper.Upload(ctx, UploadParams{File: headers[i], Topic: topics[i]}, 1, 0)
			if err != nil {
				results <- outcome{idx: i, err: err}
				return
			}
			att, ok := got.(Attachment)
			if !ok {
				results <- outcome{idx: i, err: fmt.Errorf("unexpected result type %T", got)}
				return
			}
			results <- outcome{idx: i, att: att}
		}(i)
	}
	wg.Wait()
	close(results)

	seen := make(map[int]Attachment, workers)
	for r := range results {
		require.NoError(t, r.err, "worker %d upload failed", r.idx)
		seen[r.idx] = r.att
	}
	require.Len(t, seen, workers)
	for i := 0; i < workers; i++ {
		att := seen[i]
		require.Equal(t, topics[i], att.Topic, "worker %d topic crossed with another request", i)
		require.Equal(t, fmt.Sprintf("%x", sha1.Sum(contents[i])), att.Sha1, "worker %d sha1 belongs to another file", i)
		disk, err := os.ReadFile(utils.RootPath() + "/public" + att.URL)
		require.NoError(t, err)
		require.Equal(t, contents[i], disk, "worker %d on-disk content mismatched", i)
	}
}
