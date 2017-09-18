package server

import (
	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
)

// errorTagService wraps a distribution.TagService for a particular repo.
// before delegating, it ensures auth completed and there were no errors relevant to the repo.
type errorTagService struct {
	tags distribution.TagService
	repo *repository
}

var _ distribution.TagService = &errorTagService{}

func (t *errorTagService) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
	if err := t.repo.checkPendingErrors(ctx); err != nil {
		return distribution.Descriptor{}, err
	}
	return t.tags.Get(ctx, tag)
}

func (t *errorTagService) Tag(ctx context.Context, tag string, desc distribution.Descriptor) error {
	if err := t.repo.checkPendingErrors(ctx); err != nil {
		return err
	}
	return t.tags.Tag(ctx, tag, desc)
}

func (t *errorTagService) Untag(ctx context.Context, tag string) error {
	if err := t.repo.checkPendingErrors(ctx); err != nil {
		return err
	}
	return t.tags.Untag(ctx, tag)
}

func (t *errorTagService) All(ctx context.Context) ([]string, error) {
	if err := t.repo.checkPendingErrors(ctx); err != nil {
		return nil, err
	}
	return t.tags.All(ctx)
}

func (t *errorTagService) Lookup(ctx context.Context, digest distribution.Descriptor) ([]string, error) {
	if err := t.repo.checkPendingErrors(ctx); err != nil {
		return nil, err
	}
	return t.tags.Lookup(ctx, digest)
}
