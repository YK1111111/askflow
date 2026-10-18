package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterMetricsRoute 暴露 Prometheus 文本格式的内部进程指标。
func RegisterMetricsRoute(router *gin.Engine, handler http.Handler) {
	router.GET("/metrics", gin.WrapH(handler))
}
