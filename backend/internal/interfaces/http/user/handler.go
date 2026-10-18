package userhttp

import (
	"context"
	"errors"
	"net/http"

	applicationuser "askflow/internal/application/user"
	domainuser "askflow/internal/domain/user"
	"askflow/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type userRegister interface {
	Register_User(ctx context.Context, input applicationuser.RegisterInput) (applicationuser.RegisterOutput, error)
}

type userProfile interface {
	Get_UserProfile(ctx context.Context, userID uint64) (applicationuser.ProfileOutput, error)
	Update_UserProfile(ctx context.Context, userID uint64, input applicationuser.UpdateProfileInput) (applicationuser.ProfileOutput, error)
}

// RegisterProtectedRoutes 注册需要登录的用户路由。
func RegisterProtectedRoutes(router *gin.RouterGroup, service userProfile) {
	router.GET("/users/me", func(c *gin.Context) {
		userID := c.MustGet(middleware.UserIDKey).(uint64)
		output, err := service.Get_UserProfile(c.Request.Context(), userID)
		if err != nil {
			writeProfileError(c, err)
			return
		}
		c.JSON(http.StatusOK, output)
	})

	router.POST("/users/me", func(c *gin.Context) {
		var request updateUserProfileRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数不正确"})
			return
		}

		userID := c.MustGet(middleware.UserIDKey).(uint64)
		output, err := service.Update_UserProfile(c.Request.Context(), userID, applicationuser.UpdateProfileInput{
			Nickname:  request.Nickname,
			Bio:       request.Bio,
			AvatarURL: request.AvatarURL,
		})
		if err != nil {
			writeProfileError(c, err)
			return
		}
		c.JSON(http.StatusOK, output)
	})
}

func writeProfileError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domainuser.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domainuser.ErrNicknameRequired),
		errors.Is(err, domainuser.ErrNicknameTooLong),
		errors.Is(err, domainuser.ErrBioTooLong),
		errors.Is(err, domainuser.ErrAvatarURLInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
	}
}

type userLogin interface {
	Login_User(ctx context.Context, input applicationuser.LoginInput) (applicationuser.LoginOutput, error)
}

// RegisterPublicRoutes 注册公开的用户路由。
func RegisterPublicRoutes(router *gin.RouterGroup, registerService userRegister, loginService userLogin) {
	router.POST("/users/register", func(c *gin.Context) {
		var request registerUserRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数不正确"})
			return
		}

		output, err := registerService.Register_User(c.Request.Context(), applicationuser.RegisterInput{
			Username: request.Username,
			Password: request.Password,
		})
		if err != nil {
			switch {
			case errors.Is(err, domainuser.ErrUsernameAlreadyExists):
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			case errors.Is(err, domainuser.ErrUsernameRequired), errors.Is(err, applicationuser.ErrPasswordTooShort):
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
			}
			return
		}

		c.JSON(http.StatusCreated, output)
	})

	router.POST("/users/login", func(c *gin.Context) {
		var request loginUserRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数不正确"})
			return
		}

		output, err := loginService.Login_User(c.Request.Context(), applicationuser.LoginInput{
			Username: request.Username,
			Password: request.Password,
		})
		if err != nil {
			if errors.Is(err, applicationuser.ErrInvalidCredentials) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
			return
		}

		c.JSON(http.StatusOK, output)
	})
}
