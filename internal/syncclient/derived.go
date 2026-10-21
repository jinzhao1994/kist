package syncclient

import (
	"context"
	"io"
	"net/url"

	"github.com/jinzhao1994/kist/internal/library"
)

var _ library.DerivedSource = (*Peer)(nil)

func (p *Peer) DescribeDerived(ctx context.Context, key, recipe string, retry bool) (library.SyncDerived, error) {
	var result library.SyncDerived
	err := p.json(ctx, "POST", "derived/describe", struct {
		Key    string `json:"key"`
		Recipe string `json:"recipe"`
		Retry  bool   `json:"retry"`
	}{key, recipe, retry}, &result, 1<<20)
	if err == nil {
		err = library.ValidateSyncDerived(result, key, recipe)
	}
	return result, err
}

func (p *Peer) OpenDerived(ctx context.Context, descriptor library.SyncDerived, offset int64) (io.ReadCloser, error) {
	job := descriptor.Job
	if err := library.ValidateSyncDerived(descriptor, job.BlobKey, job.Recipe); err != nil {
		return nil, err
	}
	if job.State != "ready" {
		return nil, library.ErrMediaNotReady
	}
	query := url.Values{"key": {job.BlobKey}, "recipe": {job.Recipe}, "generation": {job.Generation}, "content_key": {descriptor.ContentKey}}
	return p.openContentRange(ctx, "derived/content?"+query.Encode(), descriptor.ContentKey, job.Result.Bytes, offset)
}
