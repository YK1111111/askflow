// Package comment 编排回答评论相关的应用用例。
package comment

import (
	"context"
	"time"

	domainanswer "askflow/internal/domain/answer"
	domaincomment "askflow/internal/domain/comment"
	domainuser "askflow/internal/domain/user"
)

type PublishInput struct {
	AnswerID uint64
	AuthorID uint64
	Content  string
}

type PublishOutput struct {
	ID uint64 `json:"id"`
}

type AuthorOutput struct {
	ID        uint64 `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type CommentOutput struct {
	ID        uint64       `json:"id"`
	AnswerID  uint64       `json:"answer_id"`
	Content   string       `json:"content"`
	Author    AuthorOutput `json:"author"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

type Service struct {
	commentRepository domaincomment.Repository
	answerRepository  domainanswer.Repository
	userRepository    domainuser.Repository
}

func NewService(commentRepository domaincomment.Repository, answerRepository domainanswer.Repository, userRepository domainuser.Repository) *Service {
	return &Service{commentRepository: commentRepository, answerRepository: answerRepository, userRepository: userRepository}
}

func (s *Service) Publish_Comment(ctx context.Context, input PublishInput) (PublishOutput, error) {
	if _, err := s.answerRepository.Find_AnswerByID(ctx, input.AnswerID); err != nil {
		return PublishOutput{}, err
	}
	newComment, err := domaincomment.New(input.AnswerID, input.AuthorID, input.Content)
	if err != nil {
		return PublishOutput{}, err
	}
	if err := s.commentRepository.Create_Comment(ctx, newComment); err != nil {
		return PublishOutput{}, err
	}
	return PublishOutput{ID: newComment.ID}, nil
}

func (s *Service) List_CommentsByAnswerID(ctx context.Context, answerID uint64) ([]CommentOutput, error) {
	if _, err := s.answerRepository.Find_AnswerByID(ctx, answerID); err != nil {
		return nil, err
	}
	comments, err := s.commentRepository.Find_CommentsByAnswerID(ctx, answerID)
	if err != nil {
		return nil, err
	}
	output := make([]CommentOutput, 0, len(comments))
	for _, foundComment := range comments {
		author, err := s.userRepository.Find_UserByID(ctx, foundComment.AuthorID)
		if err != nil {
			return nil, err
		}
		output = append(output, CommentOutput{
			ID: foundComment.ID, AnswerID: foundComment.AnswerID, Content: foundComment.Content,
			Author:    AuthorOutput{ID: author.ID, Username: author.Username, Nickname: author.Nickname, AvatarURL: author.AvatarURL},
			CreatedAt: foundComment.CreatedAt, UpdatedAt: foundComment.UpdatedAt,
		})
	}
	return output, nil
}
