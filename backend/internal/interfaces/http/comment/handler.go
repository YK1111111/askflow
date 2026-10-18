package commenthttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	applicationcomment "askflow/internal/application/comment"
	domainanswer "askflow/internal/domain/answer"
	domaincomment "askflow/internal/domain/comment"
	"askflow/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type commentPublisher interface {
	Publish_Comment(context.Context, applicationcomment.PublishInput) (applicationcomment.PublishOutput, error)
}
type commentListGetter interface {
	List_CommentsByAnswerID(context.Context, uint64) ([]applicationcomment.CommentOutput, error)
}

func RegisterPublicRoutes(router *gin.RouterGroup, service commentListGetter) {
	router.GET("/answers/:id/comments", func(c *gin.Context) {
		answerID, ok := parseAnswerID(c)
		if !ok {
			return
		}
		output, err := service.List_CommentsByAnswerID(c.Request.Context(), answerID)
		if err != nil {
			writeCommentError(c, err)
			return
		}
		c.JSON(http.StatusOK, output)
	})
}

func RegisterProtectedRoutes(router *gin.RouterGroup, service commentPublisher) {
	router.POST("/answers/:id/comments", func(c *gin.Context) {
		answerID, ok := parseAnswerID(c)
		if !ok {
			return
		}
		var request publishCommentRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数不正确"})
			return
		}
		output, err := service.Publish_Comment(c.Request.Context(), applicationcomment.PublishInput{
			AnswerID: answerID, AuthorID: c.MustGet(middleware.UserIDKey).(uint64), Content: request.Content,
		})
		if err != nil {
			writeCommentError(c, err)
			return
		}
		c.JSON(http.StatusCreated, output)
	})
}

func parseAnswerID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "回答 ID 不正确"})
		return 0, false
	}
	return id, true
}

func writeCommentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domainanswer.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domaincomment.ErrAnswerRequired), errors.Is(err, domaincomment.ErrAuthorRequired),
		errors.Is(err, domaincomment.ErrContentRequired), errors.Is(err, domaincomment.ErrContentTooLong):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
	}
}
