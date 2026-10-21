package derive

import "context"

type ownedProcessGroupKey struct{}

// WithOwnedProcessGroup keeps converters in the dedicated render worker's group.
// The caller must first verify that the worker is its group leader. The parent
// owns group termination and joins all descendants before releasing scratch use.
func WithOwnedProcessGroup(ctx context.Context) context.Context {
	return context.WithValue(ctx, ownedProcessGroupKey{}, true)
}
