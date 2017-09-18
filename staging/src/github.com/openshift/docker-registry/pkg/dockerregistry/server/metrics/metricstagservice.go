package metrics

import (
	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
)

// TagService wraps a distribution.TagService to collect statistics
type TagService struct {
	Tags     distribution.TagService
	Reponame string
}

var _ distribution.TagService = &TagService{}

func (t *TagService) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
	defer NewTimer(RegistryAPIRequests, []string{"tagservice.get", t.Reponame}).Stop()
	return t.Tags.Get(ctx, tag)
}

func (t *TagService) Tag(ctx context.Context, tag string, desc distribution.Descriptor) error {
	defer NewTimer(RegistryAPIRequests, []string{"tagservice.tag", t.Reponame}).Stop()
	return t.Tags.Tag(ctx, tag, desc)
}

func (t *TagService) Untag(ctx context.Context, tag string) error {
	defer NewTimer(RegistryAPIRequests, []string{"tagservice.untag", t.Reponame}).Stop()
	return t.Tags.Untag(ctx, tag)
}

func (t *TagService) All(ctx context.Context) ([]string, error) {
	defer NewTimer(RegistryAPIRequests, []string{"tagservice.all", t.Reponame}).Stop()
	return t.Tags.All(ctx)
}

func (t *TagService) Lookup(ctx context.Context, digest distribution.Descriptor) ([]string, error) {
	defer NewTimer(RegistryAPIRequests, []string{"tagservice.lookup", t.Reponame}).Stop()
	return t.Tags.Lookup(ctx, digest)
}
