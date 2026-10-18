package answerhttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	applicationanswer "askflow/internal/application/answer"
	applicationanswerranking "askflow/internal/application/answerranking"
	domainanswer "askflow/internal/domain/answer"
	domainanswerranking "askflow/internal/domain/answerranking"
	domainquestion "askflow/internal/domain/question"
	"askflow/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type answerPublisher interface {
	Publish_Answer(ctx context.Context, input applicationanswer.PublishInput) (applicationanswer.PublishOutput, error)
}

type answerListGetter interface {
	List_RankedAnswers(ctx context.Context, questionID, anchorID uint64, cursor string, limit int) (applicationanswerranking.Output, error)
}

// RegisterPublicRoutes 注册公开的回答路由。
func RegisterPublicRoutes(router *gin.RouterGroup, service answerListGetter) {
	router.GET("/questions/:id/answers", func(c *gin.Context) {
		questionID, ok := parseQuestionID(c)
		if !ok {
			return
		}

		anchorID, ok := parseOptionalUint64(c, "anchor_answer_id")
		if !ok {
			return
		}
		limit, ok := parseLimit(c)
		if !ok {
			return
		}
		output, err := service.List_RankedAnswers(c.Request.Context(), questionID, anchorID, c.Query("cursor"), limit)
		if err != nil {
			switch {
			case errors.Is(err, domainquestion.ErrNotFound), errors.Is(err, domainanswerranking.ErrAnchorNotFound):
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			case errors.Is(err, domainanswerranking.ErrAnchorWrongQuestion), errors.Is(err, domainanswerranking.ErrInvalidCursor):
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
			}
			return
		}
		c.JSON(http.StatusOK, output)
	})
}

func parseOptionalUint64(c *gin.Context, name string) (uint64, bool) {
	value := c.Query(name)
	if value == "" {
		return 0, true
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": name + " 必须是正整数"})
		return 0, false
	}
	return parsed, true
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

// RegisterProtectedRoutes 注册需要登录的回答路由。
func RegisterProtectedRoutes(router *gin.RouterGroup, service answerPublisher) {
	router.POST("/questions/:id/answers", func(c *gin.Context) {
		questionID, ok := parseQuestionID(c)
		if !ok {
			return
		}

		var request publishAnswerRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数不正确"})
			return
		}

		userID := c.MustGet(middleware.UserIDKey).(uint64)
		output, err := service.Publish_Answer(c.Request.Context(), applicationanswer.PublishInput{
			QuestionID: questionID,
			AuthorID:   userID,
			Content:    request.Content,
		})
		if err != nil {
			switch {
			case errors.Is(err, domainquestion.ErrNotFound):
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			case errors.Is(err, domainanswer.ErrQuestionRequired),
				errors.Is(err, domainanswer.ErrAuthorRequired),
				errors.Is(err, domainanswer.ErrContentRequired),
				errors.Is(err, domainanswer.ErrContentTooLong):
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
			}
			return
		}

		c.JSON(http.StatusCreated, output)
	})
}

func parseQuestionID(c *gin.Context) (uint64, bool) {
	questionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || questionID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "问题 ID 不正确"})
		return 0, false
	}
	return questionID, true
}
