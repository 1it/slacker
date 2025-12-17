package notify

import (
	"context"

	"github.com/1it/slacker/internal/resource"
)

// Notifier is a generic notifier that can be used to notify a system.
type Notifier interface {
	Notify(ctx context.Context, resource resource.Resource) error
}

type NotifyAction struct {
	Target string
	Action string
}
