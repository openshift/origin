// +build !linux

package archive

import "github.com/docker/docker/pkg/archive"

func getWhiteoutConverter(format archive.WhiteoutFormat) tarWhiteoutConverter {
	return nil
}
