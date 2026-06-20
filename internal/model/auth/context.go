package auth

import "context"

type contextKey string

const userKey contextKey = "user"

func ContextWithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(
		ctx,
		userKey,
		user,
	)
}

func UserFromContext(ctx context.Context) (User, bool) {

	user, ok := ctx.Value(userKey).(User)

	return user, ok
}

func MustUser(ctx context.Context) User {

	user, ok := UserFromContext(ctx)

	if !ok {
		panic("user not found in context")
	}

	return user
}
