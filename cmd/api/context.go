package main

import (
	"context"
	"net/http"

	"marketier/internal/data"
)

type contextKey string

const userContextKey = contextKey("user")

func (app *application) contextSetUser(r *http.Request, user *data.BaseUserAccount) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, user)
	return r.WithContext(ctx)
}

func (app *application) contextGetUser(r *http.Request) *data.BaseUserAccount {
	user, ok := r.Context().Value(userContextKey).(*data.BaseUserAccount)
	if !ok {
		panic("missing user value in request context")
	}

	return user
}
