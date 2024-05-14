package handler

import (
	"go-build-admin/app/admin/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CurdHandler struct {
	log   *zap.Logger
	authM *model.AuthModel
}

func NewCurdHandler(log *zap.Logger, authM *model.AuthModel) *CurdHandler {
	return &CurdHandler{
		log:   log,
		authM: authM,
	}
}

// 开始生成
func (h *CurdHandler) Generate(ctx *gin.Context) {

}

// 从log开始
func (h *CurdHandler) LogStart(ctx *gin.Context) {

}

// 删除CRUD记录和生成的文件
func (h *CurdHandler) Delete(ctx *gin.Context) {

}

// 获取文件路径数据
func (h *CurdHandler) GetFileData(ctx *gin.Context) {

}

// 检查是否已有CRUD记录
func (h *CurdHandler) CheckCrudLog(ctx *gin.Context) {

}

// 解析字段数据
func (h *CurdHandler) ParseFieldData(ctx *gin.Context) {

}

// 生成前检查
func (h *CurdHandler) GenerateCheck(ctx *gin.Context) {

}

func (h *CurdHandler) DatabaseList(ctx *gin.Context) {

}

// 关联表数据解析
func (h *CurdHandler) parseJoinData(ctx *gin.Context) {

}

// 解析模型方法（设置器、获取器等）
func (h *CurdHandler) parseModelMethods(ctx *gin.Context) {

}

// 控制器/模型等文件的一些杂项属性解析
func (h *CurdHandler) parseSundryData(ctx *gin.Context) {

}

func (h *CurdHandler) getFormField(ctx *gin.Context) {

}

func (h *CurdHandler) getRemoteSelectUrl(ctx *gin.Context) {

}

func (h *CurdHandler) getTableColumn(ctx *gin.Context) {

}

func (h *CurdHandler) getColumnDict(ctx *gin.Context) {

}
