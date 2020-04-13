package transactional

import (
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
)

// ShallowStorage implements the storer.ShallowStorer for the transactional package.
type ShallowStorage struct {
	storer.ShallowStorer
	temporal storer.ShallowStorer
}

// NewShallowStorage returns a new ShallowStorage based on a base storer and
// a temporal storer.
func NewShallowStorage(base, temporal storer.ShallowStorer) *ShallowStorage {
	return &ShallowStorage{
		ShallowStorer: base,
		temporal:      temporal,
	}
}

// SetShallow honors the storer.ShallowStorer interface.
func (s *ShallowStorage) SetShallow(commits []plumbing.Hash) error {
	return s.temporal.SetShallow(commits)
}

// Shallow honors the storer.ShallowStorer interface.
func (s *ShallowStorage) Shallow() ([]plumbing.Hash, error) {
	shallow, err := s.temporal.Shallow()
	if err != nil {
		return nil, err
	}

	if len(shallow) != 0 {
		return shallow, nil
	}

	return s.ShallowStorer.Shallow()
}

// Commit it copies the shallow information of the temporal storage into the
// base storage.
func (s *ShallowStorage) Commit() error {
	commits, err := s.temporal.Shallow()
	if err != nil || len(commits) == 0 {
		return err
	}

	return s.ShallowStorer.SetShallow(commits)
}
