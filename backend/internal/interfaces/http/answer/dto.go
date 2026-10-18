package answerhttp

type publishAnswerRequest struct {
	Content string `json:"content" binding:"required"`
}
