package answerlikehttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	applicationanswerlike "askflow/internal/application/answerlike"
	domainanswer "askflow/internal/domain/answer"
	domainanswerlike "askflow/internal/domain/answerlike"
	"askflow/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type answerLikeService interface {
	Like_Answer(ctx context.Context, userID, answerID uint64) error
	Unlike_Answer(ctx context.Context, userID, answerID uint64) error
	Get_AnswerLikeStatus(ctx context.Context, userID, answerID uint64) (applicationanswerlike.StatusOutput, error)
}

// RegisterProtectedRoutes 注册需要登录的回答点赞路由。
func RegisterProtectedRoutes(router *gin.RouterGroup, service answerLikeService) {
	router.POST("/answers/:id/like", func(c *gin.Context) {
		answerID, ok := parseAnswerID(c)
		if !ok {
			return
		}
		userID := c.MustGet(middleware.UserIDKey).(uint64)
		if err := service.Like_Answer(c.Request.Context(), userID, answerID); err != nil {
			writeLikeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "点赞成功"})
	})

	router.POST("/answers/:id/unlike", func(c *gin.Context) {
		answerID, ok := parseAnswerID(c)
		if !ok {
			return
		}
		userID := c.MustGet(middleware.UserIDKey).(uint64)
		if err := service.Unlike_Answer(c.Request.Context(), userID, answerID); err != nil {
			writeLikeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "取消点赞成功"})
	})

	router.GET("/answers/:id/like-status", func(c *gin.Context) {
		answerID, ok := parseAnswerID(c)
		if !ok {
			return
		}
		userID := c.MustGet(middleware.UserIDKey).(uint64)
		output, err := service.Get_AnswerLikeStatus(c.Request.Context(), userID, answerID)
		if err != nil {
			writeLikeError(c, err)
			return
		}
		c.JSON(http.StatusOK, output)
	})
}

func parseAnswerID(c *gin.Context) (uint64, bool) {
	answerID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || answerID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "回答 ID 不正确"})
		return 0, false
	}
	return answerID, true
}

func writeLikeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domainanswer.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domainanswerlike.ErrAlreadyLiked):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, domainanswerlike.ErrUserRequired), errors.Is(err, domainanswerlike.ErrAnswerRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
	}
}
