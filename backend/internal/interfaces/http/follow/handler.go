package followhttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	applicationfollow "askflow/internal/application/follow"
	domainfollow "askflow/internal/domain/follow"
	domainuser "askflow/internal/domain/user"
	"askflow/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type followCommander interface {
	Follow_User(ctx context.Context, followerID, followeeID uint64) error
	Unfollow_User(ctx context.Context, followerID, followeeID uint64) error
	Get_FollowStatus(ctx context.Context, followerID, followeeID uint64) (applicationfollow.StatusOutput, error)
}

type followingListGetter interface {
	List_Following(ctx context.Context, followerID uint64) ([]applicationfollow.UserOutput, error)
}

// RegisterPublicRoutes 注册公开的关注关系路由。
func RegisterPublicRoutes(router *gin.RouterGroup, service followingListGetter) {
	router.GET("/users/:id/following", func(c *gin.Context) {
		userID, ok := parseUserID(c)
		if !ok {
			return
		}
		output, err := service.List_Following(c.Request.Context(), userID)
		if err != nil {
			writeFollowError(c, err)
			return
		}
		c.JSON(http.StatusOK, output)
	})
}

// RegisterProtectedRoutes 注册需要登录的关注关系路由。
func RegisterProtectedRoutes(router *gin.RouterGroup, service followCommander) {
	router.POST("/users/:id/follow", func(c *gin.Context) {
		followeeID, ok := parseUserID(c)
		if !ok {
			return
		}
		followerID := c.MustGet(middleware.UserIDKey).(uint64)
		if err := service.Follow_User(c.Request.Context(), followerID, followeeID); err != nil {
			writeFollowError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "关注成功"})
	})

	router.POST("/users/:id/unfollow", func(c *gin.Context) {
		followeeID, ok := parseUserID(c)
		if !ok {
			return
		}
		followerID := c.MustGet(middleware.UserIDKey).(uint64)
		if err := service.Unfollow_User(c.Request.Context(), followerID, followeeID); err != nil {
			writeFollowError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "取消关注成功"})
	})

	router.GET("/users/:id/follow-status", func(c *gin.Context) {
		followeeID, ok := parseUserID(c)
		if !ok {
			return
		}
		followerID := c.MustGet(middleware.UserIDKey).(uint64)
		output, err := service.Get_FollowStatus(c.Request.Context(), followerID, followeeID)
		if err != nil {
			writeFollowError(c, err)
			return
		}
		c.JSON(http.StatusOK, output)
	})
}

func parseUserID(c *gin.Context) (uint64, bool) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户 ID 不正确"})
		return 0, false
	}
	return userID, true
}

func writeFollowError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domainuser.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domainfollow.ErrAlreadyFollowing):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, domainfollow.ErrFollowerRequired),
		errors.Is(err, domainfollow.ErrFolloweeRequired),
		errors.Is(err, domainfollow.ErrCannotFollowSelf):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
	}
}
