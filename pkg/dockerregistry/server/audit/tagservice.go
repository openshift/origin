package audit

import (
	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
)

// TagService wraps a distribution.TagService to track operation result and
// write it in the audit log.
type TagService struct {
	tags   distribution.TagService
	logger *AuditLogger
}

var _ distribution.TagService = &TagService{}

func NewTagService(ctx context.Context, tags distribution.TagService) distribution.TagService {
	return &TagService{
		tags:   tags,
		logger: GetLogger(ctx),
	}
}

func (t *TagService) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
	t.logger.Log("TagService.Get")
	desc, err := t.tags.Get(ctx, tag)
	t.logger.LogResult(err, "TagService.Get")
	return desc, err
}

func (t *TagService) Tag(ctx context.Context, tag string, desc distribution.Descriptor) error {
	t.logger.Log("TagService.Tag")
	err := t.tags.Tag(ctx, tag, desc)
	t.logger.LogResult(err, "TagService.Tag")
	return err
}

func (t *TagService) Untag(ctx context.Context, tag string) error {
	t.logger.Log("TagService.Untag")
	err := t.tags.Untag(ctx, tag)
	t.logger.LogResult(err, "TagService.Untag")
	return err
}

func (t *TagService) All(ctx context.Context) ([]string, error) {
	t.logger.Log("TagService.All")
	list, err := t.tags.All(ctx)
	t.logger.LogResult(err, "TagService.All")
	return list, err
}

func (t *TagService) Lookup(ctx context.Context, digest distribution.Descriptor) ([]string, error) {
	t.logger.Log("TagService.Lookup")
	list, err := t.tags.Lookup(ctx, digest)
	t.logger.LogResult(err, "TagService.Lookup")
	return list, err
}
