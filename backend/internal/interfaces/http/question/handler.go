package questionhttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	applicationquestion "askflow/internal/application/question"
	domainquestion "askflow/internal/domain/question"
	"askflow/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type questionPublisher interface {
	Publish_Question(ctx context.Context, input applicationquestion.PublishInput) (applicationquestion.PublishOutput, error)
}

type questionDetailGetter interface {
	Get_QuestionDetail(ctx context.Context, questionID uint64) (applicationquestion.DetailOutput, error)
}

// RegisterPublicRoutes 注册公开的问题路由。
func RegisterPublicRoutes(router *gin.RouterGroup, service questionDetailGetter) {
	router.GET("/questions/:id", func(c *gin.Context) {
		questionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || questionID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "问题 ID 不正确"})
			return
		}

		output, err := service.Get_QuestionDetail(c.Request.Context(), questionID)
		if err != nil {
			if errors.Is(err, domainquestion.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
			return
		}
		c.JSON(http.StatusOK, output)
	})
}

// RegisterProtectedRoutes 注册需要登录的问题路由。
func RegisterProtectedRoutes(router *gin.RouterGroup, service questionPublisher) {
	router.POST("/questions", func(c *gin.Context) {
		var request publishQuestionRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数不正确"})
			return
		}

		userID := c.MustGet(middleware.UserIDKey).(uint64)
		output, err := service.Publish_Question(c.Request.Context(), applicationquestion.PublishInput{
			AuthorID: userID,
			Title:    request.Title,
			Content:  request.Content,
		})
		if err != nil {
			switch {
			case errors.Is(err, domainquestion.ErrAuthorRequired),
				errors.Is(err, domainquestion.ErrTitleRequired),
				errors.Is(err, domainquestion.ErrTitleTooLong),
				errors.Is(err, domainquestion.ErrContentRequired),
				errors.Is(err, domainquestion.ErrContentTooLong):
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
			}
			return
		}

		c.JSON(http.StatusCreated, output)
	})
}
