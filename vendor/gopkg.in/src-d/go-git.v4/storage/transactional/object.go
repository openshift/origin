package transactional

import (
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
)

// ObjectStorage implements the storer.EncodedObjectStorer for the transactional package.
type ObjectStorage struct {
	storer.EncodedObjectStorer
	temporal storer.EncodedObjectStorer
}

// NewObjectStorage returns a new EncodedObjectStorer based on a base storer and
// a temporal storer.
func NewObjectStorage(base, temporal storer.EncodedObjectStorer) *ObjectStorage {
	return &ObjectStorage{EncodedObjectStorer: base, temporal: temporal}
}

// SetEncodedObject honors the storer.EncodedObjectStorer interface.
func (o *ObjectStorage) SetEncodedObject(obj plumbing.EncodedObject) (plumbing.Hash, error) {
	return o.temporal.SetEncodedObject(obj)
}

// HasEncodedObject honors the storer.EncodedObjectStorer interface.
func (o *ObjectStorage) HasEncodedObject(h plumbing.Hash) error {
	err := o.EncodedObjectStorer.HasEncodedObject(h)
	if err == plumbing.ErrObjectNotFound {
		return o.temporal.HasEncodedObject(h)
	}

	return err
}

// EncodedObjectSize honors the storer.EncodedObjectStorer interface.
func (o *ObjectStorage) EncodedObjectSize(h plumbing.Hash) (int64, error) {
	sz, err := o.EncodedObjectStorer.EncodedObjectSize(h)
	if err == plumbing.ErrObjectNotFound {
		return o.temporal.EncodedObjectSize(h)
	}

	return sz, err
}

// EncodedObject honors the storer.EncodedObjectStorer interface.
func (o *ObjectStorage) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	obj, err := o.EncodedObjectStorer.EncodedObject(t, h)
	if err == plumbing.ErrObjectNotFound {
		return o.temporal.EncodedObject(t, h)
	}

	return obj, err
}

// IterEncodedObjects honors the storer.EncodedObjectStorer interface.
func (o *ObjectStorage) IterEncodedObjects(t plumbing.ObjectType) (storer.EncodedObjectIter, error) {
	baseIter, err := o.EncodedObjectStorer.IterEncodedObjects(t)
	if err != nil {
		return nil, err
	}

	temporalIter, err := o.temporal.IterEncodedObjects(t)
	if err != nil {
		return nil, err
	}

	return storer.NewMultiEncodedObjectIter([]storer.EncodedObjectIter{
		baseIter,
		temporalIter,
	}), nil
}

// Commit it copies the objects of the temporal storage into the base storage.
func (o *ObjectStorage) Commit() error {
	iter, err := o.temporal.IterEncodedObjects(plumbing.AnyObject)
	if err != nil {
		return err
	}

	return iter.ForEach(func(obj plumbing.EncodedObject) error {
		_, err := o.EncodedObjectStorer.SetEncodedObject(obj)
		return err
	})
}
