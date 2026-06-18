package githubaction

type issueComment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
}

type issueCommentRequest struct {
	Body string `json:"body"`
}
