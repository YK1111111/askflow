package answer

import (
	"context"
	"time"

	domainanswer "askflow/internal/domain/answer"
	domainquestion "askflow/internal/domain/question"
	domainuser "askflow/internal/domain/user"
)

type AuthorOutput struct {
	ID        uint64 `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type AnswerOutput struct {
	ID            uint64       `json:"id"`
	QuestionID    uint64       `json:"question_id"`
	Content       string       `json:"content"`
	Author        AuthorOutput `json:"author"`
	LikeCount     uint64       `json:"like_count"`
	CommentCount  uint64       `json:"comment_count"`
	FavoriteCount uint64       `json:"favorite_count"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// ListService 实现问题回答列表查询用例。
type ListService struct {
	answerRepository   domainanswer.Repository
	questionRepository domainquestion.Repository
	userRepository     domainuser.Repository
}

func NewListService(
	answerRepository domainanswer.Repository,
	questionRepository domainquestion.Repository,
	userRepository domainuser.Repository,
) *ListService {
	return &ListService{
		answerRepository:   answerRepository,
		questionRepository: questionRepository,
		userRepository:     userRepository,
	}
}

func (s *ListService) List_AnswersByQuestionID(ctx context.Context, questionID uint64) ([]AnswerOutput, error) {
	if _, err := s.questionRepository.Find_QuestionByID(ctx, questionID); err != nil {
		return nil, err
	}

	answers, err := s.answerRepository.Find_AnswersByQuestionID(ctx, questionID)
	if err != nil {
		return nil, err
	}

	output := make([]AnswerOutput, 0, len(answers))
	for _, foundAnswer := range answers {
		author, err := s.userRepository.Find_UserByID(ctx, foundAnswer.AuthorID)
		if err != nil {
			return nil, err
		}
		output = append(output, AnswerOutput{
			ID:            foundAnswer.ID,
			QuestionID:    foundAnswer.QuestionID,
			Content:       foundAnswer.Content,
			LikeCount:     foundAnswer.LikeCount,
			CommentCount:  foundAnswer.CommentCount,
			FavoriteCount: foundAnswer.FavoriteCount,
			Author: AuthorOutput{
				ID:        author.ID,
				Username:  author.Username,
				Nickname:  author.Nickname,
				AvatarURL: author.AvatarURL,
			},
			CreatedAt: foundAnswer.CreatedAt,
			UpdatedAt: foundAnswer.UpdatedAt,
		})
	}
	return output, nil
}
