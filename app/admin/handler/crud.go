package handler

import (
	"go-build-admin/app/admin/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CrudHandler struct {
	log   *zap.Logger
	authM *model.AuthModel
}

func NewCrudHandler(log *zap.Logger, authM *model.AuthModel) *CrudHandler {
	return &CrudHandler{
		log:   log,
		authM: authM,
	}
}

// 开始生成
func (h *CrudHandler) Generate(ctx *gin.Context) {

}

// 从log开始
func (h *CrudHandler) LogStart(ctx *gin.Context) {

}

// 删除CRUD记录和生成的文件
func (h *CrudHandler) Delete(ctx *gin.Context) {

}

// 获取文件路径数据
func (h *CrudHandler) GetFileData(ctx *gin.Context) {

}

// 检查是否已有CRUD记录
func (h *CrudHandler) CheckCrudLog(ctx *gin.Context) {

}

// 解析字段数据
func (h *CrudHandler) ParseFieldData(ctx *gin.Context) {

}

// 生成前检查
func (h *CrudHandler) GenerateCheck(ctx *gin.Context) {

}

func (h *CrudHandler) DatabaseList(ctx *gin.Context) {

}

// 关联表数据解析
func (h *CrudHandler) parseJoinData(ctx *gin.Context) {

}

// 解析模型方法（设置器、获取器等）
func (h *CrudHandler) parseModelMethods(ctx *gin.Context) {

}

// 控制器/模型等文件的一些杂项属性解析
func (h *CrudHandler) parseSundryData(ctx *gin.Context) {

}

func (h *CrudHandler) getFormField(ctx *gin.Context) {

}

func (h *CrudHandler) getRemoteSelectUrl(ctx *gin.Context) {

}

func (h *CrudHandler) getTableColumn(ctx *gin.Context) {

}

func (h *CrudHandler) getColumnDict(ctx *gin.Context) {

}
