package transactional

import (
	"io"

	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/storage"
)

// Storage is a transactional implementation of git.Storer, it demux the write
// and read operation of two separate storers, allowing to merge content calling
// Storage.Commit.
//
// The API and functionality of this package are considered EXPERIMENTAL and is
// not considered stable nor production ready.
type Storage interface {
	storage.Storer
	Commit() error
}

// basic implements the Storage interface.
type basic struct {
	s, temporal storage.Storer

	*ObjectStorage
	*ReferenceStorage
	*IndexStorage
	*ShallowStorage
	*ConfigStorage
}

// packageWriter implements storer.PackfileWriter interface over
// a Storage with a temporal storer that supports it.
type packageWriter struct {
	*basic
	pw storer.PackfileWriter
}

// NewStorage returns a new Storage based on two repositories, base is the base
// repository where the read operations are read and temporal is were all
// the write operations are stored.
func NewStorage(base, temporal storage.Storer) Storage {
	st := &basic{
		s:        base,
		temporal: temporal,

		ObjectStorage:    NewObjectStorage(base, temporal),
		ReferenceStorage: NewReferenceStorage(base, temporal),
		IndexStorage:     NewIndexStorage(base, temporal),
		ShallowStorage:   NewShallowStorage(base, temporal),
		ConfigStorage:    NewConfigStorage(base, temporal),
	}

	pw, ok := temporal.(storer.PackfileWriter)
	if ok {
		return &packageWriter{
			basic: st,
			pw:    pw,
		}
	}

	return st
}

// Module it honors the storage.ModuleStorer interface.
func (s *basic) Module(name string) (storage.Storer, error) {
	base, err := s.s.Module(name)
	if err != nil {
		return nil, err
	}

	temporal, err := s.temporal.Module(name)
	if err != nil {
		return nil, err
	}

	return NewStorage(base, temporal), nil
}

// Commit it copies the content of the temporal storage into the base storage.
func (s *basic) Commit() error {
	for _, c := range []interface{ Commit() error }{
		s.ObjectStorage,
		s.ReferenceStorage,
		s.IndexStorage,
		s.ShallowStorage,
		s.ConfigStorage,
	} {
		if err := c.Commit(); err != nil {
			return err
		}
	}

	return nil
}

// PackfileWriter honors storage.PackfileWriter.
func (s *packageWriter) PackfileWriter() (io.WriteCloser, error) {
	return s.pw.PackfileWriter()
}
