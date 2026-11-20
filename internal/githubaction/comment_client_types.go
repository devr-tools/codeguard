package githubaction

type issueComment struct {
	ID   int64       `json:"id"`
	Body string      `json:"body"`
	User commentUser `json:"user"`
}

type commentUser struct {
	Login string `json:"login"`
}

type issueCommentRequest struct {
	Body string `json:"body"`
}
