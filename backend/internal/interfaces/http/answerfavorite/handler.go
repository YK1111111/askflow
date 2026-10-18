package answerfavoritehttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	applicationanswerfavorite "askflow/internal/application/answerfavorite"
	domainanswer "askflow/internal/domain/answer"
	domainanswerfavorite "askflow/internal/domain/answerfavorite"
	"askflow/internal/interfaces/http/middleware"
	"github.com/gin-gonic/gin"
)

type service interface {
	Favorite_Answer(context.Context, uint64, uint64) error
	Unfavorite_Answer(context.Context, uint64, uint64) error
	Get_AnswerFavoriteStatus(context.Context, uint64, uint64) (applicationanswerfavorite.StatusOutput, error)
}

func RegisterProtectedRoutes(router *gin.RouterGroup, svc service) {
	router.POST("/answers/:id/favorite", func(c *gin.Context) { runCommand(c, svc.Favorite_Answer, "收藏成功") })
	router.POST("/answers/:id/unfavorite", func(c *gin.Context) { runCommand(c, svc.Unfavorite_Answer, "取消收藏成功") })
	router.GET("/answers/:id/favorite-status", func(c *gin.Context) {
		answerID, ok := parseAnswerID(c)
		if !ok {
			return
		}
		output, err := svc.Get_AnswerFavoriteStatus(c.Request.Context(), c.MustGet(middleware.UserIDKey).(uint64), answerID)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, output)
	})
}

func runCommand(c *gin.Context, command func(context.Context, uint64, uint64) error, message string) {
	answerID, ok := parseAnswerID(c)
	if !ok {
		return
	}
	if err := command(c.Request.Context(), c.MustGet(middleware.UserIDKey).(uint64), answerID); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": message})
}

func parseAnswerID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "回答 ID 不正确"})
		return 0, false
	}
	return id, true
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domainanswer.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domainanswerfavorite.ErrAlreadyFavorited):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, domainanswerfavorite.ErrUserRequired), errors.Is(err, domainanswerfavorite.ErrAnswerRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
	}
}
