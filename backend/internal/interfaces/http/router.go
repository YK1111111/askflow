// Package http 负责定义 HTTP 路由和处理器。
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewRouter 创建 AskFlow 的 HTTP 路由。
func NewRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return router
}
