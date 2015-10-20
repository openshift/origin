package server

import (
	"fmt"
	"io"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/registry/storage"
	"github.com/docker/distribution/registry/storage/driver"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/util/sets"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// graph contains functions for collecting information about orphaned objects
// and their disposal.
// These functions will only reliably work on strongly consistent storage
// systems.
// https://en.wikipedia.org/wiki/Consistency_model

// RefSet is a set of digests.
type RefSet map[digest.Digest]sets.Empty

// NewRefSet creates a new RefSet instance with given items.
func NewRefSet(ds ...digest.Digest) RefSet {
	rs := RefSet{}
	rs.Insert(ds...)
	return rs
}

// Insert adds new digest entries into a set.
func (rs RefSet) Insert(ds ...digest.Digest) {
	for _, d := range ds {
		rs[d] = sets.Empty{}
	}
}

// Has returns true if given digest is contained in the set.
func (rs RefSet) Has(d digest.Digest) bool {
	_, exists := rs[d]
	return exists
}

// Delete removes all given digests from a set if they exist.
func (rs RefSet) Delete(ds ...digest.Digest) {
	for _, d := range ds {
		delete(rs, d)
	}
}

// Difference returns a new set containing all the items in the rs set that are
// not present in the other.
func (rs RefSet) Difference(other RefSet) RefSet {
	res := NewRefSet()
	for d := range rs {
		if !other.Has(d) {
			res.Insert(d)
		}
	}
	return res
}

// Union returns a new set containting all the items from both sets.
func (rs RefSet) Union(other RefSet) RefSet {
	res := NewRefSet()
	for d := range rs {
		res.Insert(d)
	}
	for d := range other {
		res.Insert(d)
	}
	return res
}

// ManifestRevisionGraphInfo stores information about a manifest revision.
type ManifestRevisionGraphInfo struct {
	Tag        string
	Signatures []digest.Digest
}

// ManifestRevisionReferences maps digests to corresponding manifest revision objects.
type ManifestRevisionReferences map[digest.Digest]*ManifestRevisionGraphInfo

// RepositoryGraphInfo contains information about a repository.
type RepositoryGraphInfo struct {
	registryGraph *RegistryGraph
	// LayersToDelete contain digests of repository layer links that are
	// referenced only by manifest revisions marked for deletion.
	LayersToDelete RefSet
	// LayersToKeep contain digests of repository layer links that are
	// referenced by at least one manifest revision that will be preserved.
	LayersToKeep RefSet
	// ManifestsToDelete contains manifest revisions for each image in
	// OpenShift marked for deletion,
	ManifestsToDelete ManifestRevisionReferences
	// ManifestsToKeep contains manifest revisions loaded from OpenShift's etcd
	// store not marked for deletion.
	ManifestsToKeep ManifestRevisionReferences

	// TODO: collect tags
}

// IsDirty returns true if the repository contains any orphaned objects.
func (rg *RepositoryGraphInfo) IsDirty() bool {
	if len(rg.LayersToDelete) > 0 || len(rg.ManifestsToDelete) > 0 {
		return true
	}
	return false
}

// deleteLayerRefs marks given layer link and corresponding blob data
// references for deletion unless they are referenced by manifest revision to
// be kept.
func (rg *RepositoryGraphInfo) deleteLayerRefs(refs ...digest.Digest) {
	rs := NewRefSet(refs...)
	toDelete := rs.Difference(rg.LayersToKeep)
	rg.LayersToDelete = rg.LayersToDelete.Union(toDelete)
	for lRef := range toDelete {
		rg.registryGraph.deleteBlobRefs(lRef)
	}
}

// keepLayerRefs makes sure that given layer link and corresponding blob data
// references will be preserved. Overrides any prior and subsequent calls to
// DeleteLayerRefs on the same reference.
func (rg *RepositoryGraphInfo) keepLayerRefs(refs ...digest.Digest) {
	rg.LayersToDelete.Delete(refs...)
	rg.LayersToKeep.Insert(refs...)
	rg.registryGraph.keepBlobRefs(refs...)
}

// newManifestRevisionInfo creates an abstraction for manifest revision and
// marks it either for deletion or preservation.
func (rg *RepositoryGraphInfo) newManifestRevisionInfo(ref digest.Digest, tag string, toKeep bool) (*ManifestRevisionGraphInfo, bool) {
	var mi *ManifestRevisionGraphInfo
	var isNew = false

	if mi, exists := rg.ManifestsToKeep[ref]; exists {
		return mi, isNew
	}
	mi, exists := rg.ManifestsToDelete[ref]
	if exists {
		if toKeep {
			delete(rg.ManifestsToDelete, ref)
		}
	} else {
		mi = &ManifestRevisionGraphInfo{Tag: tag}
		isNew = true
	}

	if toKeep {
		rg.ManifestsToKeep[ref] = mi
		rg.registryGraph.keepBlobRefs(ref)
	} else {
		rg.ManifestsToDelete[ref] = mi
		rg.registryGraph.deleteBlobRefs(ref)
	}

	return mi, isNew
}

// RegistryGraph stores registry objects can be deleted or must be preserved.
// Any object once marked as being preserved cannot be marked for deletion,
// neither its dependencies.
type RegistryGraph struct {
	ctx    context.Context
	driver driver.StorageDriver
	reg    *registry
	// IgnoreErrors causes pruning process to continue when error occurs.
	// All the errors will be accumulated and returned.
	IgnoreErrors bool

	// BlobsToDelete holds digests of registry blobs that can be deleted from
	// registry's storage.
	BlobsToDelete RefSet
	// BlobsToKeep holds digests of registry blobs that must be preserved.
	BlobsToKeep RefSet
	// ReposToDelete maps names of repositories to their abstraction objects.
	// Corresponding image streams of contained entries were marked for
	// deletion.
	ReposToDelete map[string]*RepositoryGraphInfo
	// ReposToKeep maps names of repositories that won't be deleted.
	ReposToKeep map[string]*RepositoryGraphInfo
}

// LoadRegistryGraph walks a filesystem and OpenShift's etcd stora and collects
// all the information needed to clean up a registry. Ir returns a new instance
// of RegistryGraph.
func LoadRegistryGraph(ctx context.Context, reg *registry, driver driver.StorageDriver) (*RegistryGraph, error) {
	rg := &RegistryGraph{
		ctx:           reg.ctx,
		driver:        driver,
		reg:           reg,
		BlobsToDelete: NewRefSet(),
		BlobsToKeep:   NewRefSet(),
		ReposToDelete: make(map[string]*RepositoryGraphInfo),
		ReposToKeep:   make(map[string]*RepositoryGraphInfo),
	}

	// load imagestream deletions and their images from etcd
	rg.reg.enumRepoKind = enumRepoDeletion
	if err := rg.loadRepositories(false); err != nil {
		return nil, err
	}

	// load regular imagestreams and their images from etcd
	rg.reg.enumRepoKind = enumRepoExisting
	if err := rg.loadRepositories(true); err != nil {
		return nil, err
	}

	// load regular repositories from local storage and their images from etcd
	rg.reg.enumRepoKind = enumRepoLocal
	if err := rg.loadRepositories(true); err != nil {
		return nil, err
	}

	return rg, nil
}

// PruneOrphanedObjects deletes:
//
//  1. Manifest revisions and their signatures for their corresponding images
//     marked for deletion.
//  2. Repository layers belonging to images marked for deletion not refered
//     by any image being preserved.
//  3. Blob data refered by any layer, signature or manifest revision being deleted
//     and not refered by any being preserved.
func (rg *RegistryGraph) PruneOrphanedObjects() []error {
	errors := []error{}

	// collects all the image stream names whose repositories where successfully
	// cleaned up and thus can be removed from imageStreamDeletions
	imageStreamDeletionsToClean := make([]string, 0, len(rg.ReposToDelete))

	for name, ri := range rg.ReposToDelete {
		// delete local links and data from etcd
		es := rg.cleanRepository(name)
		errors = append(errors, es...)
		if !rg.IgnoreErrors && len(es) > 0 {
			return errors
		}
		if len(es) > 0 {
			continue
		}

		// delete repositories from local storage
		if len(ri.ManifestsToKeep) == 0 {
			v := storage.NewVacuum(rg.ctx, rg.driver)
			err := v.RemoveRepository(name)
			if err != nil {
				errors = append(errors, err)
				if !rg.IgnoreErrors {
					return errors
				}
				continue
			}
		}

		imageStreamDeletionsToClean = append(imageStreamDeletionsToClean, name)
	}

	rg.pruneImageStreamDeletions(imageStreamDeletionsToClean...)

	// clean up other repositories
	for name := range rg.ReposToKeep {
		errors = append(errors, rg.cleanRepository(name)...)
		if !rg.IgnoreErrors && len(errors) > 0 {
			return errors
		}
	}

	bd, err := storage.RegistryBlobDeleter(rg.reg.Namespace)
	if err != nil {
		return append(errors, err)
	}

	// delete orphaned blobs
	for ref := range rg.BlobsToDelete {
		err := bd.Delete(rg.ctx, ref)
		if err != nil {
			errors = append(errors, err)
			if !rg.IgnoreErrors {
				return errors
			}
		}
	}

	return nil
}

// deleteBlobRefs marks given digests of blob data for deletion unless
// they are not being preserved.
func (rg *RegistryGraph) deleteBlobRefs(refs ...digest.Digest) {
	rs := NewRefSet(refs...)
	toDelete := rs.Difference(rg.BlobsToKeep)
	rg.BlobsToDelete = rg.BlobsToDelete.Union(toDelete)
}

// KeepBlobRefs marks given digests of blob data for preservation. Overrides
// any prior and subsequent call to DeleteBlobRefs on any given digest.
func (rg *RegistryGraph) keepBlobRefs(refs ...digest.Digest) {
	rg.BlobsToDelete.Delete(refs...)
	rg.BlobsToKeep.Insert(refs...)
}

// newRepositoryInfo creates a new graph abstration for repository and marks it
// either for deletion or preservation.
func (rg *RegistryGraph) newRepositoryInfo(name string, toKeep bool) (*RepositoryGraphInfo, bool) {
	var rgi *RepositoryGraphInfo
	var isNew = false

	if rgi, exists := rg.ReposToKeep[name]; exists {
		return rgi, isNew
	}
	rgi, exists := rg.ReposToDelete[name]
	if exists {
		if toKeep {
			delete(rg.ReposToDelete, name)
		}
	} else {
		rgi = &RepositoryGraphInfo{
			registryGraph:     rg,
			LayersToDelete:    NewRefSet(),
			LayersToKeep:      NewRefSet(),
			ManifestsToDelete: make(ManifestRevisionReferences),
			ManifestsToKeep:   make(ManifestRevisionReferences),
		}
		isNew = true
	}

	if toKeep {
		rg.ReposToKeep[name] = rgi
	} else {
		rg.ReposToDelete[name] = rgi
	}

	return rgi, isNew
}

// loadRepositories iterates over repositories and collects information about
// all their manifest revisions, signatures and layers. Repository names are
// either fetched from etcd or local storage depending on registry's
// configuration. toKeep decides whether repositories will be marked for
// deletion or preservation. It has no influence on loaded manifest revisions
// and their dependent objects.
func (rg *RegistryGraph) loadRepositories(toKeep bool) error {
	const bufLen = 512
	var (
		repoNames = make([]string, bufLen)
		last      = ""
	)

	for {
		n, err := rg.reg.Repositories(rg.ctx, repoNames, last)
		if err != nil && err != io.EOF {
			return err
		}

		// success
		if err == io.EOF {
			break
		}

		for _, repoName := range repoNames[0:n] {
			if _, exists := rg.ReposToKeep[repoName]; exists {
				continue
			}
			if _, exists := rg.ReposToDelete[repoName]; exists {
				continue
			}

			repo, err := rg.reg.Repository(rg.ctx, repoName)
			if err != nil {
				log.Warnf("Failed to load repository %q: %v", repoName, err)
				continue
			}
			_, err = rg.loadRepository(repoName, repo.(*repository), toKeep)
			if err != nil {
				log.Warnf("Failed to load repository %q: %v", repoName, err)
				continue
			}
		}

		last = repoNames[n-1]
	}

	return nil
}

// loadRepository loads objects contained in given repository. And marks it
// either for deletion or preservation based on toKeep.
func (rg *RegistryGraph) loadRepository(repoName string, repo *repository, toKeep bool) (*RepositoryGraphInfo, error) {
	ri, isNew := rg.newRepositoryInfo(repoName, toKeep)
	if !isNew {
		// all information already loaded
		return ri, nil
	}

	for _, kind := range []fields.Selector{
		// first enumerate images marked for deletion
		enumManifestKindToDelete,
		// then the regular ones
		enumManifestKindToKeep} {

		manServ, err := repo.Manifests(rg.ctx,
			makeGetCheckImageStreamOption(false),
			makeChangeEnumKindOption(kind))
		if err != nil {
			return nil, err
		}

		manRefs, err := manServ.Enumerate()
		if err != nil {
			return nil, err
		}

		for _, ref := range manRefs {
			err := rg.loadManifestRevision(repo, repoName, ref, kind == enumManifestKindToKeep)
			if err != nil {
				log.Warnf("Failed to load manifest revision \"%s@%s\": %v", repoName, ref, err)
			}
		}
	}

	return ri, nil
}

// loadManifestRevision	loads manifest from etcd and marks it either for
// deletion or preservation based on toKeep. All the dependent objects inherit
// this attribute if they are loaded for the first time.
func (rg *RegistryGraph) loadManifestRevision(repo distribution.Repository, repoName string, ref digest.Digest, toKeep bool) error {
	var ri *RepositoryGraphInfo
	manServ, err := repo.Manifests(rg.ctx, makeGetCheckImageStreamOption(false))
	manifest, err := manServ.Get(ref)
	if err != nil {
		return err
	}

	ri, exists := rg.ReposToKeep[repoName]
	if !exists {
		ri, exists = rg.ReposToDelete[repoName]
		if !exists {
			return fmt.Errorf("cannot load manifest revision from unknown repository %q", repoName)
		}
	}

	mi, _ := ri.newManifestRevisionInfo(ref, manifest.Tag, toKeep)

	for _, layer := range manifest.FSLayers {
		if toKeep {
			ri.keepLayerRefs(layer.BlobSum)
		} else {
			ri.deleteLayerRefs(layer.BlobSum)
		}
	}

	signatures, err := repo.Signatures().Enumerate(ref)
	if err != nil {
		log.Warnf("Failed to list signatures of %s: %v", repoName+"@"+ref.String(), err)
	} else {
		if toKeep {
			rg.keepBlobRefs(signatures...)
		} else {
			rg.deleteBlobRefs(signatures...)
		}
		mi.Signatures = signatures
	}

	return nil
}

// cleanRepository removes orphaned layers, signatures and manifest revisions
// from repository. Manifests will be deleted from etcd and the rest from
// registry's storage. It doesn't remove the repository itself.
func (rg *RegistryGraph) cleanRepository(repoName string) []error {
	var (
		errors []error
		ri     *RepositoryGraphInfo
	)

	repo, err := rg.reg.Repository(rg.ctx, repoName)
	if err != nil {
		return []error{err}
	}
	manServ, err := repo.Manifests(rg.ctx)
	if err != nil {
		return []error{err}
	}

	ri, exists := rg.ReposToKeep[repoName]
	if !exists {
		ri, exists = rg.ReposToDelete[repoName]
	}
	if ri == nil || !ri.IsDirty() {
		return errors
	}

	for mRef, mi := range ri.ManifestsToDelete {
		log.Infof("Deleting manifest revision %s:%s@%s with %d signatures", repoName, mi.Tag, mRef, len(mi.Signatures))
		err := manServ.Delete(mRef)
		if err != nil {
			log.Errorf("Failed to delete manifest revision %s:%s@%s: %v", repoName, mi.Tag, mRef, err)
			errors = append(errors, err)
			if !rg.IgnoreErrors {
				return errors
			}
			// keep all dependent layers and signatures if the delete failed
			rg.loadManifestRevision(repo, repoName, mRef, true)
		}
	}

	blobStore := repo.Blobs(rg.ctx)
	for lRef := range ri.LayersToDelete {
		err := blobStore.Delete(rg.reg.ctx, lRef)
		if err != nil {
			errors = append(errors, err)
			if !rg.IgnoreErrors && err != distribution.ErrBlobUnknown {
				return errors
			}
			// don't delete layer data if a deletion of its link failed
			rg.keepBlobRefs(lRef)
		}
	}

	return errors
}

// pruneImageStreamDeletions removes image stream deletions from OpenShift's
// etcd store. Only names of repositories successfully purged should be among
// arguments.
func (rg *RegistryGraph) pruneImageStreamDeletions(names ...string) []error {
	errors := []error{}

	for _, name := range names {
		nameParts := strings.SplitN(name, "/", 2)
		if len(nameParts) != 2 {
			errors = append(errors, fmt.Errorf("invalid repository name %q: it must be of the format <project>/<name>", name))
			continue
		}
		log.Debugf("Deleting image stream deletion %s", name)
		deletionName := imageapi.DeletionNameForImageStream(nameParts[0], nameParts[1])
		err := rg.reg.osClient.ImageStreamDeletions().Delete(deletionName)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to delete image stream deletion %s: %v", name, err))
			if !rg.IgnoreErrors {
				return errors
			}
		}
	}

	return errors
}
