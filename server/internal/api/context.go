package api

import (
	"context"

	"github.com/netodrive/server/internal/auth"
)

func withClaims(ctx context.Context, c *auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

func claimsFrom(r interface{ Context() context.Context }) *auth.Claims {
	c, _ := r.Context().Value(claimsKey).(*auth.Claims)
	return c
}
