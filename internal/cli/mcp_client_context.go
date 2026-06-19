package cli

import "context"

type clientCallerCtxKey struct{}

func withClientCaller(ctx context.Context, c clientCaller) context.Context {
	if c == nil {
		return ctx
	}
	return context.WithValue(ctx, clientCallerCtxKey{}, c)
}

func clientCallerFrom(ctx context.Context) clientCaller {
	c, _ := ctx.Value(clientCallerCtxKey{}).(clientCaller)
	return c
}
