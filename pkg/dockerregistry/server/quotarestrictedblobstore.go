// Package server wraps repository and blob store objects of docker/distribution upstream. Most significantly,
// the wrappers cause manifests to be stored in OpenShift's etcd store instead of registry's storage.
// Registry's middleware API is utilized to register the object factories.
//
// Module with quotaRestrictedBlobStore defines a wrapper for upstream blob store that does an image quota and
// limits check before committing image layer to a registry. Master server contains admission check that will
// refuse the manifest if the image exceeds whatever quota or limit set. But the check occurs too late (after
// the layers are written). This addition allows us to refuse the layers and thus keep the storage clean.
//
// *Note*: Here, we take into account just a single layer, not the image as a whole because the layers are
// uploaded before the manifest. This leads to a situation where several layers can be written until a big
// enough layer will be received that exceeds the limit.
package server

import (
	"fmt"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"

	kapi "k8s.io/kubernetes/pkg/api"

	imageadmission "github.com/openshift/origin/pkg/image/admission"
)

// newQuotaEnforcingConfig creates a configuration for quotaRestrictedBlobStore.
func newQuotaEnforcingConfig(ctx context.Context, enforceQuota string, options map[string]interface{}) *quotaEnforcingConfig {
	buildOptionValues := func(optionName string, override string) []string {
		optValues := []string{}
		if value, ok := options[optionName]; ok {
			var res string
			switch v := value.(type) {
			case string:
				res = v
			case bool:
				res = fmt.Sprintf("%t", v)
			default:
				res = fmt.Sprintf("%v", v)
			}
			optValues = append(optValues, res)
		}
		optValues = append(optValues, override)
		return optValues
	}

	enforce := false
	for _, s := range buildOptionValues("enforcequota", enforceQuota) {
		enforce = s == "true"
	}
	if !enforce {
		context.GetLogger(ctx).Info("quota enforcement disabled")
	}
	return &quotaEnforcingConfig{
		enforcementDisabled: !enforce,
	}
}

// quotaEnforcingConfig holds configuration for quotaRestrictedBlobStore.
type quotaEnforcingConfig struct {
	// if set, disables quota enforcement
	enforcementDisabled bool
}

// quotaRestrictedBlobStore wraps upstream blob store with a guard preventing big layers exceeding image quotas
// from being saved.
type quotaRestrictedBlobStore struct {
	distribution.BlobStore

	repo *repository
}

var _ distribution.BlobStore = &quotaRestrictedBlobStore{}

// Create wraps returned blobWriter with quota guard wrapper.
func (bs *quotaRestrictedBlobStore) Create(ctx context.Context) (distribution.BlobWriter, error) {
	context.GetLogger(ctx).Debug("(*quotaRestrictedBlobStore).Create: starting")

	bw, err := bs.BlobStore.Create(ctx)
	if err != nil {
		return nil, err
	}

	repo := (*bs.repo)
	repo.ctx = ctx
	return &quotaRestrictedBlobWriter{
		BlobWriter: bw,
		repo:       &repo,
	}, nil
}

// Resume wraps returned blobWriter with quota guard wrapper.
func (bs *quotaRestrictedBlobStore) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	context.GetLogger(ctx).Debug("(*quotaRestrictedBlobStore).Resume: starting")

	bw, err := bs.BlobStore.Resume(ctx, id)
	if err != nil {
		return nil, err
	}

	repo := (*bs.repo)
	repo.ctx = ctx
	return &quotaRestrictedBlobWriter{
		BlobWriter: bw,
		repo:       &repo,
	}, nil
}

// quotaRestrictedBlobWriter wraps upstream blob writer with a guard preventig big layers exceeding image
// quotas from being written.
type quotaRestrictedBlobWriter struct {
	distribution.BlobWriter

	repo *repository
}

func (bw *quotaRestrictedBlobWriter) Commit(ctx context.Context, provisional distribution.Descriptor) (canonical distribution.Descriptor, err error) {
	context.GetLogger(ctx).Debug("(*quotaRestrictedBlobWriter).Commit: starting")

	if err := admitBlobWrite(ctx, bw.repo, provisional.Size); err != nil {
		return distribution.Descriptor{}, err
	}

	return bw.BlobWriter.Commit(ctx, provisional)
}

// admitBlobWrite checks whether the blob does not exceed image limit ranges if set. Returns ErrAccessDenied
// error if the limit is exceeded.
func admitBlobWrite(ctx context.Context, repo *repository, size int64) error {
	if size < 1 {
		return nil
	}

	lrs, err := repo.limitClient.LimitRanges(repo.namespace).List(kapi.ListOptions{})
	if err != nil {
		context.GetLogger(ctx).Errorf("failed to list limitranges: %v", err)
		return err
	}

	for _, limitrange := range lrs.Items {
		context.GetLogger(ctx).Debugf("processing limit range %s/%s", limitrange.Namespace, limitrange.Name)
		for _, limit := range limitrange.Spec.Limits {
			if err := imageadmission.AdmitImage(size, limit); err != nil {
				context.GetLogger(ctx).Errorf("refusing to write blob exceeding limit range %s: %s", limitrange.Name, err.Error())
				return distribution.ErrAccessDenied
			}
		}
	}

	// TODO(1): admit also against openshift.io/ImageStream quota resource when we have image stream cache in the
	// registry
	// TODO(2): admit also against openshift.io/imagestreamimages and openshift.io/imagestreamtags resources once
	// we have image stream cache in the registry

	return nil
}
