package images

import (
	"github.com/containers/storage"
	"github.com/pkg/errors"
)

// UntagImage removes the tag from the given image
func UntagImage(store storage.Store, image *storage.Image, imgArg string) (string, error) {
	// Remove name from image.Names and set the new names
	newNames := []string{}
	removedName := ""
	for _, name := range image.Names {
		if MatchesReference(name, imgArg) {
			removedName = name
			continue
		}
		newNames = append(newNames, name)
	}
	if removedName != "" {
		if err := store.SetNames(image.ID, newNames); err != nil {
			return "", errors.Wrapf(err, "error removing name %q from image %q", removedName, image.ID)
		}
	}
	return removedName, nil
}

// RemoveImage removes the given image from storage
func RemoveImage(image *storage.Image, store storage.Store) (string, error) {
	_, err := store.DeleteImage(image.ID, true)
	if err != nil {
		return "", errors.Wrapf(err, "could not remove image %q", image.ID)
	}
	return image.ID, nil
}
