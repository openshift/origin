package v1

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"

	"github.com/openshift/origin/pkg/image/apis/image/v1"
	scheme "github.com/openshift/origin/pkg/image/generated/clientset/scheme"
)

type ImageStreamExpansion interface {
	Instantiate(tag *v1.ImageStreamTagInstantiate, r io.Reader) (*v1.ImageStreamTag, error)
}

func (c *imageStreams) Instantiate(tag *v1.ImageStreamTagInstantiate, r io.Reader) (*v1.ImageStreamTag, error) {
	if r == nil {
		result := &v1.ImageStreamTag{}
		err := c.client.Post().Namespace(c.ns).Resource("imageStreamTags").Name(tag.Name).SubResource("instantiate").Body(tag).Do().Into(result)
		return result, err
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		headers := make(textproto.MIMEHeader)
		headers.Set("Content-Type", "application/json")
		w, err := mw.CreatePart(headers)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if err := scheme.Codecs.LegacyCodec(v1.SchemeGroupVersion).Encode(tag, w); err != nil {
			pw.CloseWithError(err)
			return
		}
		headers = make(textproto.MIMEHeader)
		headers.Set("Content-Type", "application/vnd.docker.image.rootfs.diff.tar.gzip")
		w, err = mw.CreatePart(headers)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(w, r); err != nil {
			pw.CloseWithError(err)
			return
		}
		if err := mw.Close(); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
		return
	}()

	result := &v1.ImageStreamTag{}
	err := c.client.Post().Namespace(c.ns).Resource("imageStreamTags").Name(tag.Name).
		SetHeader("Content-Type", mw.FormDataContentType()).
		SubResource("instantiatelayer").Body(pr).
		Do().Into(result)
	if err != nil {
		return nil, fmt.Errorf("unable to clone the image: %v", err)
	}
	return result, err
}
