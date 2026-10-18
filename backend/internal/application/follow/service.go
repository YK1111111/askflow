// Package follow 编排用户关注相关的应用用例。
package follow

import (
	"context"

	domainfollow "askflow/internal/domain/follow"
	domainuser "askflow/internal/domain/user"
)

type StatusOutput struct {
	Following bool `json:"following"`
}

type UserOutput struct {
	ID        uint64 `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Bio       string `json:"bio"`
	AvatarURL string `json:"avatar_url"`
}

// Service 实现关注、取消关注、状态查询和关注列表用例。
type Service struct {
	followRepository domainfollow.Repository
	userRepository   domainuser.Repository
}

func NewService(followRepository domainfollow.Repository, userRepository domainuser.Repository) *Service {
	return &Service{followRepository: followRepository, userRepository: userRepository}
}

func (s *Service) Follow_User(ctx context.Context, followerID, followeeID uint64) error {
	newFollow, err := domainfollow.New(followerID, followeeID)
	if err != nil {
		return err
	}
	if _, err := s.userRepository.Find_UserByID(ctx, followeeID); err != nil {
		return err
	}
	return s.followRepository.Create_Follow(ctx, newFollow)
}

func (s *Service) Unfollow_User(ctx context.Context, followerID, followeeID uint64) error {
	if _, err := domainfollow.New(followerID, followeeID); err != nil {
		return err
	}
	return s.followRepository.Delete_Follow(ctx, followerID, followeeID)
}

func (s *Service) Get_FollowStatus(ctx context.Context, followerID, followeeID uint64) (StatusOutput, error) {
	if _, err := domainfollow.New(followerID, followeeID); err != nil {
		return StatusOutput{}, err
	}
	following, err := s.followRepository.Exists_Follow(ctx, followerID, followeeID)
	if err != nil {
		return StatusOutput{}, err
	}
	return StatusOutput{Following: following}, nil
}

func (s *Service) List_Following(ctx context.Context, followerID uint64) ([]UserOutput, error) {
	if _, err := s.userRepository.Find_UserByID(ctx, followerID); err != nil {
		return nil, err
	}
	ids, err := s.followRepository.Find_FolloweeIDsByFollowerID(ctx, followerID)
	if err != nil {
		return nil, err
	}

	output := make([]UserOutput, 0, len(ids))
	for _, userID := range ids {
		foundUser, err := s.userRepository.Find_UserByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		output = append(output, UserOutput{
			ID:        foundUser.ID,
			Username:  foundUser.Username,
			Nickname:  foundUser.Nickname,
			Bio:       foundUser.Bio,
			AvatarURL: foundUser.AvatarURL,
		})
	}
	return output, nil
}
