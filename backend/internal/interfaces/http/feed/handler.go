// Package feedhttp 提供 Feed HTTP 接口。
package feedhttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	applicationfeed "askflow/internal/application/feed"
	"askflow/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type followingFeedGetter interface {
	Get_FollowingFeed(ctx context.Context, userID uint64, cursor string, limit int) (applicationfeed.FollowingOutput, error)
}

type hotFeedGetter interface {
	Get_HotFeed(ctx context.Context, cursor string, limit int) (applicationfeed.HotOutput, error)
}

type recommendedFeedGetter interface {
	Get_RecommendedFeed(ctx context.Context, userID uint64, cursor string, limit int) (applicationfeed.RecommendedOutput, error)
}

// RegisterRecommendedRoutes 注册支持匿名访问和登录个性化的推荐 Feed 路由。
func RegisterRecommendedRoutes(router *gin.RouterGroup, service recommendedFeedGetter) {
	router.GET("/feeds/recommended", func(c *gin.Context) {
		limit, ok := parseLimit(c)
		if !ok {
			return
		}
		userID, _ := c.Get(middleware.UserIDKey)
		var currentUserID uint64
		if userID != nil {
			currentUserID, _ = userID.(uint64)
		}
		output, err := service.Get_RecommendedFeed(c.Request.Context(), currentUserID, c.Query("cursor"), limit)
		if err != nil {
			if errors.Is(err, applicationfeed.ErrInvalidCursor) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
			return
		}
		c.JSON(http.StatusOK, output)
	})
}

// RegisterPublicRoutes 注册公开的热门 Feed 路由。
func RegisterPublicRoutes(router *gin.RouterGroup, service hotFeedGetter) {
	router.GET("/feeds/hot", func(c *gin.Context) {
		limit, ok := parseLimit(c)
		if !ok {
			return
		}
		output, err := service.Get_HotFeed(c.Request.Context(), c.Query("cursor"), limit)
		if err != nil {
			if errors.Is(err, applicationfeed.ErrInvalidCursor) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
			return
		}
		c.JSON(http.StatusOK, output)
	})
}

// RegisterProtectedRoutes 注册需要登录的 Feed 路由。
func RegisterProtectedRoutes(router *gin.RouterGroup, service followingFeedGetter) {
	router.GET("/feeds/following", func(c *gin.Context) {
		limit, ok := parseLimit(c)
		if !ok {
			return
		}
		userID := c.MustGet(middleware.UserIDKey).(uint64)
		output, err := service.Get_FollowingFeed(c.Request.Context(), userID, c.Query("cursor"), limit)
		if err != nil {
			if errors.Is(err, applicationfeed.ErrInvalidCursor) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
			return
		}
		c.JSON(http.StatusOK, output)
	})
}

func parseLimit(c *gin.Context) (int, bool) {
	value := c.Query("limit")
	if value == "" {
		return 0, true
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit 必须是正整数"})
		return 0, false
	}
	return limit, true
}
