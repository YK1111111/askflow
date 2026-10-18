package question

import (
	"context"
	"time"

	domainquestion "askflow/internal/domain/question"
	domainquestionview "askflow/internal/domain/questionview"
	domainuser "askflow/internal/domain/user"
)

type AuthorOutput struct {
	ID        uint64 `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type DetailOutput struct {
	ID        uint64       `json:"id"`
	Title     string       `json:"title"`
	Content   string       `json:"content"`
	Author    AuthorOutput `json:"author"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// DetailService 实现问题详情查询用例。
type DetailService struct {
	questionRepository domainquestion.Repository
	userRepository     domainuser.Repository
	viewRepository     domainquestionview.Repository
}

func NewDetailService(
	questionRepository domainquestion.Repository,
	userRepository domainuser.Repository,
	viewRepository domainquestionview.Repository,
) *DetailService {
	return &DetailService{
		questionRepository: questionRepository,
		userRepository:     userRepository,
		viewRepository:     viewRepository,
	}
}

func (s *DetailService) Get_QuestionDetail(ctx context.Context, questionID uint64) (DetailOutput, error) {
	foundQuestion, err := s.questionRepository.Find_QuestionByID(ctx, questionID)
	if err != nil {
		return DetailOutput{}, err
	}
	author, err := s.userRepository.Find_UserByID(ctx, foundQuestion.AuthorID)
	if err != nil {
		return DetailOutput{}, err
	}
	// 浏览统计失败不影响问题正文读取，热门 Feed 可在后续请求中自然恢复。
	_ = s.viewRepository.Record_QuestionView(ctx, questionID)

	return DetailOutput{
		ID:      foundQuestion.ID,
		Title:   foundQuestion.Title,
		Content: foundQuestion.Content,
		Author: AuthorOutput{
			ID:        author.ID,
			Username:  author.Username,
			Nickname:  author.Nickname,
			AvatarURL: author.AvatarURL,
		},
		CreatedAt: foundQuestion.CreatedAt,
		UpdatedAt: foundQuestion.UpdatedAt,
	}, nil
}
