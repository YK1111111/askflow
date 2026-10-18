package answerfavoritehttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	applicationanswerfavorite "askflow/internal/application/answerfavorite"
	"askflow/internal/interfaces/http/middleware"
	"github.com/gin-gonic/gin"
)

type fakeService struct{}

func (fakeService) Favorite_Answer(context.Context, uint64, uint64) error   { return nil }
func (fakeService) Unfavorite_Answer(context.Context, uint64, uint64) error { return nil }
func (fakeService) Get_AnswerFavoriteStatus(context.Context, uint64, uint64) (applicationanswerfavorite.StatusOutput, error) {
	return applicationanswerfavorite.StatusOutput{Favorited: true}, nil
}

func TestFavoriteHandlers(t *testing.T) {
	router := gin.New()
	api := router.Group("/api/v1")
	api.Use(func(c *gin.Context) { c.Set(middleware.UserIDKey, uint64(1)); c.Next() })
	RegisterProtectedRoutes(api, fakeService{})
	for _, test := range []struct{ method, path string }{{http.MethodPost, "/api/v1/answers/2/favorite"}, {http.MethodPost, "/api/v1/answers/2/unfavorite"}, {http.MethodGet, "/api/v1/answers/2/favorite-status"}} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("接口 %s 状态码为 %d", test.path, response.Code)
		}
	}
}
