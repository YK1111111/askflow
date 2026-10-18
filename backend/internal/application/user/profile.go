package user

import (
	"context"

	domainuser "askflow/internal/domain/user"
)

type ProfileOutput struct {
	ID        uint64 `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Bio       string `json:"bio"`
	AvatarURL string `json:"avatar_url"`
}

type UpdateProfileInput struct {
	Nickname  string
	Bio       string
	AvatarURL string
}

// ProfileService 实现当前用户资料查询与修改用例。
type ProfileService struct {
	repository domainuser.Repository
}

func NewProfileService(repository domainuser.Repository) *ProfileService {
	return &ProfileService{repository: repository}
}

func (s *ProfileService) Get_UserProfile(ctx context.Context, userID uint64) (ProfileOutput, error) {
	foundUser, err := s.repository.Find_UserByID(ctx, userID)
	if err != nil {
		return ProfileOutput{}, err
	}
	return user2ProfileOutput(foundUser), nil
}

func (s *ProfileService) Update_UserProfile(
	ctx context.Context,
	userID uint64,
	input UpdateProfileInput,
) (ProfileOutput, error) {
	foundUser, err := s.repository.Find_UserByID(ctx, userID)
	if err != nil {
		return ProfileOutput{}, err
	}
	if err := foundUser.Update_Profile(input.Nickname, input.Bio, input.AvatarURL); err != nil {
		return ProfileOutput{}, err
	}
	if err := s.repository.Update_User_Profile(ctx, foundUser); err != nil {
		return ProfileOutput{}, err
	}
	return user2ProfileOutput(foundUser), nil
}

func user2ProfileOutput(user *domainuser.User) ProfileOutput {
	return ProfileOutput{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		Bio:       user.Bio,
		AvatarURL: user.AvatarURL,
	}
}
