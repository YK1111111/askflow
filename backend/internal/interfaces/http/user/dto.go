package userhttp

type registerUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type updateUserProfileRequest struct {
	Nickname  string `json:"nickname" binding:"required"`
	Bio       string `json:"bio"`
	AvatarURL string `json:"avatar_url"`
}
