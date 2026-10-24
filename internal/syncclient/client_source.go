package syncclient

import (
	"context"
	"io"

	"github.com/jinzhao1994/kist/internal/library"
)

// ClientSource resolves connection configuration only when a transfer is
// needed. Cached or locally generated offline content requires no credentials.
type ClientSource struct{ client *library.Client }

func NewClientSource(client *library.Client) *ClientSource { return &ClientSource{client: client} }
func (s *ClientSource) peer(ctx context.Context) (*Peer, error) {
	return NewForClient(ctx, s.client)
}
func (s *ClientSource) Capabilities(ctx context.Context) (library.SyncCapabilities, error) {
	peer, err := s.peer(ctx)
	if err != nil {
		return library.SyncCapabilities{}, err
	}
	return peer.Capabilities(ctx)
}
func (s *ClientSource) OpenOriginal(ctx context.Context, key string, size, offset int64) (io.ReadCloser, error) {
	peer, err := s.peer(ctx)
	if err != nil {
		return nil, err
	}
	return peer.OpenOriginal(ctx, key, size, offset)
}
func (s *ClientSource) DescribeDerived(ctx context.Context, key, recipe string, retry bool) (library.SyncDerived, error) {
	peer, err := s.peer(ctx)
	if err != nil {
		return library.SyncDerived{}, err
	}
	return peer.DescribeDerived(ctx, key, recipe, retry)
}
func (s *ClientSource) OpenDerived(ctx context.Context, d library.SyncDerived, offset int64) (io.ReadCloser, error) {
	peer, err := s.peer(ctx)
	if err != nil {
		return nil, err
	}
	return peer.OpenDerived(ctx, d, offset)
}
