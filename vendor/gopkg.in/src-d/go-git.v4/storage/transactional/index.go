package transactional

import (
	"gopkg.in/src-d/go-git.v4/plumbing/format/index"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
)

// IndexStorage implements the storer.IndexStorage for the transactional package.
type IndexStorage struct {
	storer.IndexStorer
	temporal storer.IndexStorer

	set bool
}

// NewIndexStorage returns a new IndexStorer based on a base storer and a
// temporal storer.
func NewIndexStorage(s, temporal storer.IndexStorer) *IndexStorage {
	return &IndexStorage{
		IndexStorer: s,
		temporal:    temporal,
	}
}

// SetIndex honors the storer.IndexStorer interface.
func (s *IndexStorage) SetIndex(idx *index.Index) (err error) {
	if err := s.temporal.SetIndex(idx); err != nil {
		return err
	}

	s.set = true
	return nil
}

// Index honors the storer.IndexStorer interface.
func (s *IndexStorage) Index() (*index.Index, error) {
	if !s.set {
		return s.IndexStorer.Index()
	}

	return s.temporal.Index()
}

// Commit it copies the index from the temporal storage into the base storage.
func (s *IndexStorage) Commit() error {
	if !s.set {
		return nil
	}

	idx, err := s.temporal.Index()
	if err != nil {
		return err
	}

	return s.IndexStorer.SetIndex(idx)
}
