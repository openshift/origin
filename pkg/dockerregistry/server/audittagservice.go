package server

import (
	"github.com/docker/distribution"
	"github.com/docker/distribution/context"

	"github.com/openshift/origin/pkg/dockerregistry/server/audit"
)

// auditTagService wraps a distribution.TagService to track operation result and
// write it in the audit log.
type auditTagService struct {
	tags distribution.TagService
}

var _ distribution.TagService = &auditTagService{}

func (t *auditTagService) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
	audit.GetLogger(ctx).Log("TagService.Get")
	desc, err := t.tags.Get(ctx, tag)
	audit.GetLogger(ctx).LogResult(err, "TagService.Get")
	return desc, err
}

func (t *auditTagService) Tag(ctx context.Context, tag string, desc distribution.Descriptor) error {
	audit.GetLogger(ctx).Log("TagService.Tag")
	err := t.tags.Tag(ctx, tag, desc)
	audit.GetLogger(ctx).LogResult(err, "TagService.Tag")
	return err
}

func (t *auditTagService) Untag(ctx context.Context, tag string) error {
	audit.GetLogger(ctx).Log("TagService.Untag")
	err := t.tags.Untag(ctx, tag)
	audit.GetLogger(ctx).LogResult(err, "TagService.Untag")
	return err
}

func (t *auditTagService) All(ctx context.Context) ([]string, error) {
	audit.GetLogger(ctx).Log("TagService.All")
	list, err := t.tags.All(ctx)
	audit.GetLogger(ctx).LogResult(err, "TagService.All")
	return list, err
}

func (t *auditTagService) Lookup(ctx context.Context, digest distribution.Descriptor) ([]string, error) {
	audit.GetLogger(ctx).Log("TagService.Lookup")
	list, err := t.tags.Lookup(ctx, digest)
	audit.GetLogger(ctx).LogResult(err, "TagService.Lookup")
	return list, err
}
