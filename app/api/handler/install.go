package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type InstallHandler struct {
	log *zap.Logger
}

func NewInstallHandler(log *zap.Logger) *InstallHandler {
	return &InstallHandler{log: log}
}

// 命令执行窗口
func (h *InstallHandler) Terminal(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *InstallHandler) ChangePackageManager(ctx *gin.Context) {

	Success(ctx, "")
}

// 环境基础检查
func (h *InstallHandler) EnvBaseCheck(ctx *gin.Context) {

	Success(ctx, "")
}

// npm环境检查
func (h *InstallHandler) EnvNpmCheck(ctx *gin.Context) {

	Success(ctx, "")
}

// 测试数据库连接
func (h *InstallHandler) TestDatabase(ctx *gin.Context) {

	Success(ctx, "")
}

/**
 * 系统基础配置
 * post请求=开始安装
 */
func (h *InstallHandler) BaseConfig(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *InstallHandler) isInstallComplete(ctx *gin.Context) {

	Success(ctx, "")
}

// 标记命令执行完毕
func (h *InstallHandler) CommandExecComplete(ctx *gin.Context) {

	Success(ctx, "")
}

// 获取命令执行检查的结果
func (h *InstallHandler) CommandExecutionCheck(ctx *gin.Context) {

	Success(ctx, "")
}

// 安装指引
func (h *InstallHandler) ManualInstall(ctx *gin.Context) {

	Success(ctx, "")
}

// 安装指引
func (h *InstallHandler) MvDist(ctx *gin.Context) {

	Success(ctx, "")
}

// 目录是否可写
func (h *InstallHandler) writableStateDescribe(ctx *gin.Context) {

	Success(ctx, "")
}

// 数据库连接-获取数据表列表
func (h *InstallHandler) connectDb(ctx *gin.Context) {

	Success(ctx, "")
}
