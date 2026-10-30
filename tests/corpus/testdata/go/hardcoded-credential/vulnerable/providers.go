// Package fixtures holds synthetic provider-format credentials. Every value is
// fake. The AWS key is split across two literals so no contiguous AWS-shaped
// token is committed; the scanner folds adjacent literals before matching.
package fixtures

const (
	awsAccessKeyID = "AKIA" + "Q7RMZLK4TP2XWDVB"
	githubToken    = "ghp_000000000000000000000000000000000000"
	slackBotToken  = "xoxb-FAKEFAKEFAKE-FAKEFAKEFAKEFAKEFAKE"
	stripeSecret   = "sk_live_FAKEFAKEFAKEFAKEFAKE00"
)
