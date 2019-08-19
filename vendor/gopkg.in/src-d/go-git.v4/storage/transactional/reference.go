package transactional

import (
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/storage"
)

// ReferenceStorage implements the storer.ReferenceStorage for the transactional package.
type ReferenceStorage struct {
	storer.ReferenceStorer
	temporal storer.ReferenceStorer

	// deleted, remaining references at this maps are going to be deleted when
	// commit is requested, the entries are added when RemoveReference is called
	// and deleted if SetReference is called.
	deleted map[plumbing.ReferenceName]struct{}
	// packRefs if true PackRefs is going to be called in the based storer when
	// commit is called.
	packRefs bool
}

// NewReferenceStorage returns a new ReferenceStorer based on a base storer and
// a temporal storer.
func NewReferenceStorage(base, temporal storer.ReferenceStorer) *ReferenceStorage {
	return &ReferenceStorage{
		ReferenceStorer: base,
		temporal:        temporal,

		deleted: make(map[plumbing.ReferenceName]struct{}, 0),
	}
}

// SetReference honors the storer.ReferenceStorer interface.
func (r *ReferenceStorage) SetReference(ref *plumbing.Reference) error {
	delete(r.deleted, ref.Name())
	return r.temporal.SetReference(ref)
}

// SetReference honors the storer.ReferenceStorer interface.
func (r *ReferenceStorage) CheckAndSetReference(ref, old *plumbing.Reference) error {
	if old == nil {
		return r.SetReference(ref)
	}

	tmp, err := r.temporal.Reference(old.Name())
	if err == plumbing.ErrReferenceNotFound {
		tmp, err = r.ReferenceStorer.Reference(old.Name())
	}

	if err != nil {
		return err
	}

	if tmp.Hash() != old.Hash() {
		return storage.ErrReferenceHasChanged
	}

	return r.SetReference(ref)
}

// Reference honors the storer.ReferenceStorer interface.
func (r ReferenceStorage) Reference(n plumbing.ReferenceName) (*plumbing.Reference, error) {
	if _, deleted := r.deleted[n]; deleted {
		return nil, plumbing.ErrReferenceNotFound
	}

	ref, err := r.temporal.Reference(n)
	if err == plumbing.ErrReferenceNotFound {
		return r.ReferenceStorer.Reference(n)
	}

	return ref, err
}

// IterReferences honors the storer.ReferenceStorer interface.
func (r ReferenceStorage) IterReferences() (storer.ReferenceIter, error) {
	baseIter, err := r.ReferenceStorer.IterReferences()
	if err != nil {
		return nil, err
	}

	temporalIter, err := r.temporal.IterReferences()
	if err != nil {
		return nil, err
	}

	return storer.NewMultiReferenceIter([]storer.ReferenceIter{
		baseIter,
		temporalIter,
	}), nil
}

// CountLooseRefs honors the storer.ReferenceStorer interface.
func (r ReferenceStorage) CountLooseRefs() (int, error) {
	tc, err := r.temporal.CountLooseRefs()
	if err != nil {
		return -1, err
	}

	bc, err := r.ReferenceStorer.CountLooseRefs()
	if err != nil {
		return -1, err
	}

	return tc + bc, nil
}

// PackRefs honors the storer.ReferenceStorer interface.
func (r ReferenceStorage) PackRefs() error {
	r.packRefs = true
	return nil
}

// RemoveReference honors the storer.ReferenceStorer interface.
func (r ReferenceStorage) RemoveReference(n plumbing.ReferenceName) error {
	r.deleted[n] = struct{}{}
	return r.temporal.RemoveReference(n)
}

// Commit it copies the reference information of the temporal storage into the
// base storage.
func (r ReferenceStorage) Commit() error {
	for name := range r.deleted {
		if err := r.ReferenceStorer.RemoveReference(name); err != nil {
			return err
		}
	}

	iter, err := r.temporal.IterReferences()
	if err != nil {
		return err
	}

	return iter.ForEach(func(ref *plumbing.Reference) error {
		return r.ReferenceStorer.SetReference(ref)
	})
}
