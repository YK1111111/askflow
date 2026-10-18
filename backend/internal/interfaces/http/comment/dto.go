package commenthttp

type publishCommentRequest struct {
	Content string `json:"content" binding:"required"`
}
