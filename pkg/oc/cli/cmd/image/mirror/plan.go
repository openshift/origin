package mirror

import (
	"fmt"
	"io"
	"sort"
	"sync"
	"text/tabwriter"

	"github.com/docker/distribution"

	units "github.com/docker/go-units"
	godigest "github.com/opencontainers/go-digest"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/image/apis/image/reference"
)

type retrieverError struct {
	src, dst reference.DockerImageReference
	err      error
}

func (e retrieverError) Error() string {
	return e.err.Error()
}

type repositoryWork struct {
	registry   *registryPlan
	repository *repositoryPlan
	stats      struct {
		mountOpportunities int
	}
}

func (w *repositoryWork) calculateStats(existing sets.String) sets.String {
	blobs := sets.NewString()
	for i := range w.repository.blobs {
		blobs.Insert(w.repository.blobs[i].blobs.UnsortedList()...)
	}
	w.stats.mountOpportunities = blobs.Intersection(existing).Len()
	return blobs
}

type phase struct {
	independent []repositoryWork

	lock   sync.Mutex
	failed bool
}

func (p *phase) Failed() {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.failed = true
}

func (p *phase) IsFailed() bool {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.failed
}

func (p *phase) calculateStats(existingBlobs map[string]sets.String) {
	blobs := make(map[string]sets.String)
	for i, work := range p.independent {
		blobs[work.registry.name] = p.independent[i].calculateStats(existingBlobs[work.registry.name]).Union(blobs[work.registry.name])
	}
	for name, registryBlobs := range blobs {
		existingBlobs[name] = existingBlobs[name].Union(registryBlobs)
	}
}

type workPlan struct {
	phases []phase

	lock  sync.Mutex
	stats struct {
		bytes int64
	}
}

func (w *workPlan) calculateStats() {
	blobs := make(map[string]sets.String)
	for i := range w.phases {
		w.phases[i].calculateStats(blobs)
	}
}

func (w *workPlan) BytesCopied(bytes int64) {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.stats.bytes += bytes
}

func (w *workPlan) Print(out io.Writer) {
	tabw := tabwriter.NewWriter(out, 0, 0, 1, ' ', 0)
	for i := range w.phases {
		phase := &w.phases[i]
		fmt.Fprintf(out, "phase %d:\n", i)
		for _, unit := range phase.independent {
			fmt.Fprintf(tabw, "  %s\t%s\tblobs=%d\tmounts=%d\tmanifests=%d\tshared=%d\n", unit.registry.name, unit.repository.name, unit.repository.stats.sharedCount+unit.repository.stats.uniqueCount, unit.stats.mountOpportunities, unit.repository.manifests.stats.count, unit.repository.stats.sharedCount)
		}
		tabw.Flush()
	}
}

type plan struct {
	lock       sync.Mutex
	registries map[string]*registryPlan
	errs       []error
	blobs      map[godigest.Digest]distribution.Descriptor
	manifests  map[godigest.Digest]distribution.Manifest

	work *workPlan

	stats struct {
	}
}

func newPlan() *plan {
	return &plan{
		registries: make(map[string]*registryPlan),
		manifests:  make(map[godigest.Digest]distribution.Manifest),
		blobs:      make(map[godigest.Digest]distribution.Descriptor),
	}
}

func (p *plan) AddError(errs ...error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.errs = append(p.errs, errs...)
}

func (p *plan) RegistryPlan(name string) *registryPlan {
	p.lock.Lock()
	defer p.lock.Unlock()

	plan, ok := p.registries[name]
	if ok {
		return plan
	}
	plan = &registryPlan{
		parent:      p,
		name:        name,
		blobsByRepo: make(map[godigest.Digest]string),
	}
	p.registries[name] = plan
	return plan
}

func (p *plan) CacheManifest(digest godigest.Digest, manifest distribution.Manifest) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if _, ok := p.manifests[digest]; ok {
		return
	}
	p.manifests[digest] = manifest
}

func (p *plan) GetManifest(digest godigest.Digest) (distribution.Manifest, bool) {
	p.lock.Lock()
	defer p.lock.Unlock()

	existing, ok := p.manifests[digest]
	return existing, ok
}

func (p *plan) CacheBlob(blob distribution.Descriptor) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if existing, ok := p.blobs[blob.Digest]; ok && existing.Size > 0 {
		return
	}
	p.blobs[blob.Digest] = blob
}

func (p *plan) GetBlob(digest godigest.Digest) distribution.Descriptor {
	p.lock.Lock()
	defer p.lock.Unlock()

	return p.blobs[digest]
}

func (p *plan) RegistryNames() sets.String {
	p.lock.Lock()
	defer p.lock.Unlock()

	names := sets.NewString()
	for name := range p.registries {
		names.Insert(name)
	}
	return names
}

func (p *plan) Errors() []error {
	var errs []error
	for _, r := range p.registries {
		for _, repo := range r.repositories {
			errs = append(errs, repo.errs...)
		}
	}
	errs = append(errs, p.errs...)
	return errs
}

func (p *plan) BlobDescriptors(blobs sets.String) []distribution.Descriptor {
	descriptors := make([]distribution.Descriptor, 0, len(blobs))
	for s := range blobs {
		if desc, ok := p.blobs[godigest.Digest(s)]; ok {
			descriptors = append(descriptors, desc)
		} else {
			descriptors = append(descriptors, distribution.Descriptor{
				Digest: godigest.Digest(s),
			})
		}
	}
	return descriptors
}

func (p *plan) Print(w io.Writer) {
	for _, name := range p.RegistryNames().List() {
		r := p.registries[name]
		fmt.Fprintf(w, "%s/\n", name)
		for _, repoName := range r.RepositoryNames().List() {
			repo := r.repositories[repoName]
			fmt.Fprintf(w, "  %s\n", repoName)
			for _, err := range repo.errs {
				fmt.Fprintf(w, "    error: %s\n", err)
			}
			for _, blob := range repo.blobs {
				fmt.Fprintf(w, "    blobs:\n")
				blobs := p.BlobDescriptors(blob.blobs)
				sort.Slice(blobs, func(i, j int) bool {
					if blobs[i].Size == blobs[j].Size {
						return blobs[i].Digest.String() < blobs[j].Digest.String()
					}
					return blobs[i].Size < blobs[j].Size
				})
				for _, b := range blobs {
					if size := b.Size; size > 0 {
						fmt.Fprintf(w, "      %s %s %s\n", blob.fromRef, b.Digest, units.BytesSize(float64(size)))
					} else {
						fmt.Fprintf(w, "      %s %s\n", blob.fromRef, b.Digest)
					}
				}
			}
			fmt.Fprintf(w, "    manifests:\n")
			for _, s := range repo.manifests.digestCopies {
				fmt.Fprintf(w, "      %s\n", s)
			}
			for _, digest := range repo.manifests.inputDigests().List() {
				tags := repo.manifests.digestsToTags[godigest.Digest(digest)]
				for _, s := range tags.List() {
					fmt.Fprintf(w, "      %s -> %s\n", digest, s)
				}
			}
		}
		totalSize := r.stats.uniqueSize + r.stats.sharedSize
		if totalSize > 0 {
			fmt.Fprintf(w, "  stats: shared=%d unique=%d size=%s ratio=%.2f\n", r.stats.sharedCount, r.stats.uniqueCount, units.BytesSize(float64(totalSize)), float32(r.stats.uniqueSize)/float32(totalSize))
		} else {
			fmt.Fprintf(w, "  stats: shared=%d unique=%d size=%s\n", r.stats.sharedCount, r.stats.uniqueCount, units.BytesSize(float64(totalSize)))
		}
	}
}

func (p *plan) trim() {
	for name, registry := range p.registries {
		if registry.trim() {
			delete(p.registries, name)
		}
	}
}

func (p *plan) calculateStats() {
	for _, registry := range p.registries {
		registry.calculateStats()
	}
}

type registryPlan struct {
	parent *plan
	name   string

	lock         sync.Mutex
	repositories map[string]*repositoryPlan
	blobsByRepo  map[godigest.Digest]string

	stats struct {
		uniqueSize  int64
		sharedSize  int64
		uniqueCount int32
		sharedCount int32
	}
}

func (p *registryPlan) AssociateBlob(digest godigest.Digest, repo string) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.blobsByRepo[digest] = repo
}

func (p *registryPlan) MountFrom(digest godigest.Digest) (string, bool) {
	p.lock.Lock()
	defer p.lock.Unlock()

	repo, ok := p.blobsByRepo[digest]
	return repo, ok
}

func (p *registryPlan) RepositoryNames() sets.String {
	p.lock.Lock()
	defer p.lock.Unlock()

	names := sets.NewString()
	for name := range p.repositories {
		names.Insert(name)
	}
	return names
}

func (p *registryPlan) RepositoryPlan(name string) *repositoryPlan {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.repositories == nil {
		p.repositories = make(map[string]*repositoryPlan)
	}
	plan, ok := p.repositories[name]
	if ok {
		return plan
	}
	plan = &repositoryPlan{
		parent:        p,
		name:          name,
		existingBlobs: sets.NewString(),
		absentBlobs:   sets.NewString(),
	}
	p.repositories[name] = plan
	return plan
}

func (p *registryPlan) trim() bool {
	for name, plan := range p.repositories {
		if plan.trim() {
			delete(p.repositories, name)
		}
	}
	return len(p.repositories) == 0
}

func (p *registryPlan) calculateStats() {
	counts := make(map[string]int)
	for _, plan := range p.repositories {
		plan.blobCounts(counts)
	}
	for _, plan := range p.repositories {
		plan.calculateStats(counts)
	}
	for digest, count := range counts {
		if count > 1 {
			p.stats.sharedSize += p.parent.GetBlob(godigest.Digest(digest)).Size
			p.stats.sharedCount++
		} else {
			p.stats.uniqueSize += p.parent.GetBlob(godigest.Digest(digest)).Size
			p.stats.uniqueCount++
		}
	}
}

type repositoryPlan struct {
	parent *registryPlan
	name   string

	lock          sync.Mutex
	existingBlobs sets.String
	absentBlobs   sets.String
	blobs         []*repositoryBlobCopy
	manifests     *repositoryManifestPlan
	errs          []error

	stats struct {
		size        int64
		sharedSize  int64
		uniqueSize  int64
		sharedCount int32
		uniqueCount int32
	}
}

func (p *repositoryPlan) AddError(errs ...error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.errs = append(p.errs, errs...)
}

func (p *repositoryPlan) Blobs(from reference.DockerImageReference, t DestinationType, location string) *repositoryBlobCopy {
	p.lock.Lock()
	defer p.lock.Unlock()

	for _, blob := range p.blobs {
		if blob.fromRef == from {
			return blob
		}
	}
	p.blobs = append(p.blobs, &repositoryBlobCopy{
		parent: p,

		fromRef:         from,
		toRef:           reference.DockerImageReference{Registry: p.parent.name, Name: p.name},
		destinationType: t,
		location:        location,

		blobs: sets.NewString(),
	})
	return p.blobs[len(p.blobs)-1]
}

func (p *repositoryPlan) ExpectBlob(digest godigest.Digest) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.absentBlobs.Delete(digest.String())
	p.existingBlobs.Insert(digest.String())
}

func (p *repositoryPlan) Manifests(destinationType DestinationType) *repositoryManifestPlan {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.manifests == nil {
		p.manifests = &repositoryManifestPlan{
			parent:          p,
			toRef:           reference.DockerImageReference{Registry: p.parent.name, Name: p.name},
			destinationType: destinationType,
			digestsToTags:   make(map[godigest.Digest]sets.String),
			digestCopies:    sets.NewString(),
		}
	}
	return p.manifests
}

func (p *repositoryPlan) blobCounts(registryCounts map[string]int) {
	for i := range p.blobs {
		for digest := range p.blobs[i].blobs {
			registryCounts[digest]++
		}
	}
}

func (p *repositoryPlan) trim() bool {
	var blobs []*repositoryBlobCopy
	for _, blob := range p.blobs {
		if blob.trim() {
			continue
		}
		blobs = append(blobs, blob)
	}
	p.blobs = blobs
	if p.manifests != nil {
		if p.manifests.trim() {
			p.manifests = nil
		}
	}
	return len(p.blobs) == 0 && p.manifests == nil
}

func (p *repositoryPlan) calculateStats(registryCounts map[string]int) {
	p.manifests.calculateStats()
	blobs := sets.NewString()
	for i := range p.blobs {
		for digest := range p.blobs[i].blobs {
			blobs.Insert(digest)
		}
		p.blobs[i].calculateStats()
		p.stats.size += p.blobs[i].stats.size
	}
	for digest := range blobs {
		count := registryCounts[digest]
		if count > 1 {
			p.stats.sharedSize += p.parent.parent.GetBlob(godigest.Digest(digest)).Size
			p.stats.sharedCount++
		} else {
			p.stats.uniqueSize += p.parent.parent.GetBlob(godigest.Digest(digest)).Size
			p.stats.uniqueCount++
		}
	}
}

type repositoryBlobCopy struct {
	parent          *repositoryPlan
	fromRef         reference.DockerImageReference
	toRef           reference.DockerImageReference
	destinationType DestinationType
	location        string

	lock  sync.Mutex
	from  distribution.BlobService
	to    distribution.BlobService
	blobs sets.String

	stats struct {
		size        int64
		averageSize int64
	}
}

func (p *repositoryBlobCopy) AlreadyExists(blob distribution.Descriptor) {
	p.parent.parent.parent.CacheBlob(blob)
	p.parent.parent.AssociateBlob(blob.Digest, p.parent.name)
	p.parent.ExpectBlob(blob.Digest)

	p.lock.Lock()
	defer p.lock.Unlock()

	p.blobs.Delete(blob.Digest.String())
}

func (p *repositoryBlobCopy) Copy(blob distribution.Descriptor, from, to distribution.BlobService) {
	p.parent.parent.parent.CacheBlob(blob)

	p.lock.Lock()
	defer p.lock.Unlock()

	if p.from == nil {
		p.from = from
	}
	if p.to == nil {
		p.to = to
	}
	p.blobs.Insert(blob.Digest.String())
}

func (p *repositoryBlobCopy) trim() bool {
	return len(p.blobs) == 0
}

func (p *repositoryBlobCopy) calculateStats() {
	for digest := range p.blobs {
		p.stats.size += p.parent.parent.parent.GetBlob(godigest.Digest(digest)).Size
	}
	if len(p.blobs) > 0 {
		p.stats.averageSize = p.stats.size / int64(len(p.blobs))
	}
}

type repositoryManifestPlan struct {
	parent          *repositoryPlan
	toRef           reference.DockerImageReference
	destinationType DestinationType

	lock    sync.Mutex
	to      distribution.ManifestService
	toBlobs distribution.BlobService

	digestsToTags map[godigest.Digest]sets.String
	digestCopies  sets.String

	stats struct {
		count int
	}
}

func (p *repositoryManifestPlan) Copy(srcDigest godigest.Digest, srcManifest distribution.Manifest, tags []string, to distribution.ManifestService, toBlobs distribution.BlobService) {
	p.parent.parent.parent.CacheManifest(srcDigest, srcManifest)

	p.lock.Lock()
	defer p.lock.Unlock()

	if p.to == nil {
		p.to = to
	}
	if p.toBlobs == nil {
		p.toBlobs = toBlobs
	}

	if len(tags) == 0 {
		p.digestCopies.Insert(srcDigest.String())
		return
	}
	allTags := p.digestsToTags[srcDigest]
	if allTags == nil {
		allTags = sets.NewString()
		p.digestsToTags[srcDigest] = allTags
	}
	allTags.Insert(tags...)
}

func (p *repositoryManifestPlan) inputDigests() sets.String {
	p.lock.Lock()
	defer p.lock.Unlock()

	names := sets.NewString()
	for digest := range p.digestsToTags {
		names.Insert(digest.String())
	}
	return names
}

func (p *repositoryManifestPlan) trim() bool {
	for digest, tags := range p.digestsToTags {
		if len(tags) == 0 {
			delete(p.digestsToTags, digest)
		}
	}
	return len(p.digestCopies) == 0 && len(p.digestsToTags) == 0
}

func (p *repositoryManifestPlan) calculateStats() {
	p.stats.count += len(p.digestCopies)
	for _, tags := range p.digestsToTags {
		p.stats.count += len(tags)
	}
}

// Greedy turns a plan into parallizable work by taking one repo at a time. It guarantees
// that no two phases in the plan attempt to upload the same blob at the same time. In the
// worst case each phase has one unit of work.
func Greedy(plan *plan) *workPlan {
	remaining := make(map[string]map[string]repositoryWork)
	for name, registry := range plan.registries {
		work := make(map[string]repositoryWork)
		remaining[name] = work
		for repoName, repository := range registry.repositories {
			work[repoName] = repositoryWork{
				registry:   registry,
				repository: repository,
			}
		}
	}

	alreadyUploaded := make(map[string]sets.String)

	var phases []phase
	for len(remaining) > 0 {
		var independent []repositoryWork
		for name, registry := range remaining {
			// we can always take any repository that has no shared layers
			if found := takeIndependent(registry); len(found) > 0 {
				independent = append(independent, found...)
			}
			exists := alreadyUploaded[name]
			if exists == nil {
				exists = sets.NewString()
				alreadyUploaded[name] = exists
			}

			// take the most shared repositories and any that don't overlap with it
			independent = append(independent, takeMostSharedWithoutOverlap(registry, exists)...)
			if len(registry) == 0 {
				delete(remaining, name)
			}
		}
		for _, work := range independent {
			repositoryPlanAddAllExcept(work.repository, alreadyUploaded[work.registry.name], nil)
		}
		phases = append(phases, phase{independent: independent})
	}
	work := &workPlan{
		phases: phases,
	}
	work.calculateStats()
	return work
}

func takeIndependent(all map[string]repositoryWork) []repositoryWork {
	var work []repositoryWork
	for k, v := range all {
		if v.repository.stats.sharedCount == 0 {
			delete(all, k)
			work = append(work, v)
		}
	}
	return work
}

type keysWithCount struct {
	name  string
	count int
}

// takeMostSharedWithoutOverlap is a greedy algorithm that finds the repositories with the
// most shared layers that do not overlap. It will always return at least one unit of work.
func takeMostSharedWithoutOverlap(all map[string]repositoryWork, alreadyUploaded sets.String) []repositoryWork {
	keys := make([]keysWithCount, 0, len(all))
	for k, v := range all {
		keys = append(keys, keysWithCount{name: k, count: int(v.repository.stats.sharedCount)})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].count > keys[j].count })

	// from the set of possible work, ordered from most shared to least shared, take:
	// 1. the first available unit of work
	// 2. any other unit of work that does not have overlapping shared blobs
	uploadingBlobs := sets.NewString()
	var work []repositoryWork
	for _, key := range keys {
		name := key.name
		next, ok := all[name]
		if !ok {
			continue
		}
		if repositoryPlanHasAnyBlobs(next.repository, uploadingBlobs) {
			continue
		}
		repositoryPlanAddAllExcept(next.repository, uploadingBlobs, alreadyUploaded)
		delete(all, name)
		work = append(work, next)
	}
	return work
}

func repositoryPlanAddAllExcept(plan *repositoryPlan, blobs sets.String, ignore sets.String) {
	for i := range plan.blobs {
		for key := range plan.blobs[i].blobs {
			if !ignore.Has(key) {
				blobs.Insert(key)
			}
		}
	}
}

func repositoryPlanHasAnyBlobs(plan *repositoryPlan, blobs sets.String) bool {
	for i := range plan.blobs {
		if stringsIntersects(blobs, plan.blobs[i].blobs) {
			return true
		}
	}
	return false
}

func stringsIntersects(a, b sets.String) bool {
	for key := range a {
		if _, ok := b[key]; ok {
			return true
		}
	}
	return false
}

func takeOne(all map[string]repositoryWork) []repositoryWork {
	for k, v := range all {
		delete(all, k)
		return []repositoryWork{v}
	}
	return nil
}
