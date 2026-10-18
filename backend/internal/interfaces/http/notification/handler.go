// Package notificationhttp 提供通知查询、已读操作和 SSE 实时推送接口。
package notificationhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	applicationnotification "askflow/internal/application/notification"
	"askflow/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type service interface {
	List(ctx context.Context, userID, afterID uint64, limit int) (applicationnotification.ListOutput, error)
	MarkRead(ctx context.Context, userID, notificationID uint64) error
	MarkAllRead(ctx context.Context, userID uint64) error
	Subscribe(userID uint64) (<-chan applicationnotification.Output, func())
}

// RegisterProtectedRoutes 注册通知相关登录接口。
func RegisterProtectedRoutes(router *gin.RouterGroup, service service) {
	router.GET("/notifications", func(c *gin.Context) {
		userID := c.MustGet(middleware.UserIDKey).(uint64)
		afterID, _ := strconv.ParseUint(c.Query("after_id"), 10, 64)
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		output, err := service.List(c.Request.Context(), userID, afterID, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询通知失败"})
			return
		}
		c.JSON(http.StatusOK, output)
	})

	router.POST("/notifications/:id/read", func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "通知 ID 不正确"})
			return
		}
		if err := service.MarkRead(c.Request.Context(), c.MustGet(middleware.UserIDKey).(uint64), id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "通知不存在"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "更新通知失败"})
			}
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "已标记为已读"})
	})

	router.POST("/notifications/read-all", func(c *gin.Context) {
		if err := service.MarkAllRead(c.Request.Context(), c.MustGet(middleware.UserIDKey).(uint64)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新通知失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "全部标记为已读"})
	})

	router.GET("/notifications/stream", func(c *gin.Context) {
		stream(c, service, c.MustGet(middleware.UserIDKey).(uint64))
	})
}

func stream(c *gin.Context, service service, userID uint64) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器不支持 SSE"})
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	events, unsubscribe := service.Subscribe(userID)
	defer unsubscribe()
	if _, err := fmt.Fprint(c.Writer, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()
	lastID, _ := strconv.ParseUint(c.GetHeader("Last-Event-ID"), 10, 64)
	if lastID > 0 {
		replay, err := service.List(c.Request.Context(), userID, lastID, 100)
		if err != nil {
			return
		}
		for _, value := range replay.Items {
			if err := writeEvent(c, value); err != nil {
				return
			}
		}
		flusher.Flush()
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case value, ok := <-events:
			if !ok {
				return
			}
			if err := writeEvent(c, value); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(c.Writer, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeEvent(c *gin.Context, value applicationnotification.Output) error {
	content, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.Writer, "id: %d\nevent: notification\ndata: %s\n\n", value.ID, content)
	return err
}
