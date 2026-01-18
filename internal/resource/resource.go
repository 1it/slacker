package resource

import (
	"context"
	"fmt"

	"github.com/1it/slacker/internal/executor"
	"github.com/1it/slacker/internal/manifest"
)

// Handler is a generic resource that can be applied to a system.
type Handler interface {
	ID() string
	Type() string
	NeedsChange(ctx context.Context, exec executor.Executor) (bool, error)
	Apply(ctx context.Context, exec executor.Executor) (changed bool, err error)
	Notifies() string
}

// FromManifest creates a resource Handler from a manifest Resource definition
func FromManifest(r manifest.Resource) (Handler, error) {
	switch r.Type {
	case "file":
		return NewFileHandler(r), nil
	case "package":
		return NewPackageHandler(r), nil
	case "service":
		return NewServiceHandler(r), nil
	case "exec":
		return NewExecHandler(r), nil
	case "user":
		return NewUserHandler(r), nil
	default:
		return nil, fmt.Errorf("unknown resource type: %s", r.Type)
	}
}
