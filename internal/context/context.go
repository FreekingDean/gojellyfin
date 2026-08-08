package context

import (
	"context"
)

const (
	userIDKey = "userIDKey"
	actionKey = "actionKey"
)

type Context interface {
	context.Context

	WithUserID(userID string) Context
	UserID() string

	WithAction(action string) Context
	Action() string
}

type contextImpl struct {
	context.Context
}

func Background() Context {
	return &contextImpl{context.Background()}
}

func (c *contextImpl) WithUserID(userID string) Context {
	return &contextImpl{context.WithValue(c, userIDKey, userID)}
}

func (c *contextImpl) UserID() string {
	return zeroVal[string](c, userIDKey)
}

func (c *contextImpl) WithAction(action string) Context {
	return &contextImpl{context.WithValue(c, actionKey, action)}
}

func (c *contextImpl) Action() string {
	return zeroVal[string](c, actionKey)
}

func zeroVal[T any](c *contextImpl, key interface{}) T {
	if action, ok := c.Value(actionKey).(T); ok {
		return action
	}
	var zero T
	return zero
}
