// Package user 保存用户领域模型和业务规则。
package user

import (
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrUsernameRequired = errors.New("用户名不能为空")
	ErrPasswordRequired = errors.New("密码哈希不能为空")
	ErrNicknameRequired = errors.New("昵称不能为空")
	ErrNicknameTooLong  = errors.New("昵称不能超过 50 个字符")
	ErrBioTooLong       = errors.New("简介不能超过 200 个字符")
	ErrAvatarURLInvalid = errors.New("头像地址格式不正确")
)

// User 是用户领域实体。
type User struct {
	ID           uint64
	Username     string
	PasswordHash string
	Nickname     string
	Bio          string
	AvatarURL    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// New 创建一个尚未持久化的新用户。
func New(username, passwordHash string) (*User, error) {
	username = strings.TrimSpace(username)

	if username == "" {
		return nil, ErrUsernameRequired
	}
	if passwordHash == "" {
		return nil, ErrPasswordRequired
	}

	now := time.Now()
	return &User{
		Username:     username,
		PasswordHash: passwordHash,
		Nickname:     username,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// Update_Profile 修改用户公开资料。
func (u *User) Update_Profile(nickname, bio, avatarURL string) error {
	nickname = strings.TrimSpace(nickname)
	bio = strings.TrimSpace(bio)
	avatarURL = strings.TrimSpace(avatarURL)

	if nickname == "" {
		return ErrNicknameRequired
	}
	if utf8.RuneCountInString(nickname) > 50 {
		return ErrNicknameTooLong
	}
	if utf8.RuneCountInString(bio) > 200 {
		return ErrBioTooLong
	}
	if avatarURL != "" {
		parsedURL, err := url.ParseRequestURI(avatarURL)
		if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
			return ErrAvatarURLInvalid
		}
	}

	u.Nickname = nickname
	u.Bio = bio
	u.AvatarURL = avatarURL
	u.UpdatedAt = time.Now()
	return nil
}
