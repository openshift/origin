package archive

import (
	"archive/tar"
	"bytes"
	"io"

	"github.com/docker/docker/pkg/archive"
)

// TransformFileFunc is given a chance to transform an arbitrary input file.
type TransformFileFunc func(h *tar.Header, r io.Reader) ([]byte, bool, error)

// FilterArchive transforms the provided input archive (compressed) to a
// compressed archive, giving the fn a chance to transform arbitrary files.
func FilterArchive(r io.Reader, w io.Writer, fn TransformFileFunc) error {
	in, err := archive.DecompressStream(r)
	if err != nil {
		return err
	}
	tr := tar.NewReader(in)
	tw := tar.NewWriter(w)
	out, err := archive.CompressStream(tw, archive.Gzip)
	if err != nil {
		return err
	}

	for {
		h, err := tr.Next()
		if err == io.EOF {
			return out.Close()
		}
		if err != nil {
			return err
		}

		var body io.Reader = tr
		data, ok, err := fn(h, tr)
		if err != nil {
			return err
		}
		if ok {
			h.Size = int64(len(data))
			body = bytes.NewBuffer(data)
		}

		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if _, err := io.Copy(tw, body); err != nil {
			return err
		}
	}
}
