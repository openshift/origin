package server

import (
	"fmt"
	"time"

	"github.com/kubernetes-incubator/cri-o/pkg/storage"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// RemoveImage removes the image.
func (s *Server) RemoveImage(ctx context.Context, req *pb.RemoveImageRequest) (resp *pb.RemoveImageResponse, err error) {
	const operation = "remove_image"
	defer func() {
		recordOperation(operation, time.Now())
		recordError(operation, err)
	}()

	logrus.Debugf("RemoveImageRequest: %+v", req)
	image := ""
	img := req.GetImage()
	if img != nil {
		image = img.Image
	}
	if image == "" {
		return nil, fmt.Errorf("no image specified")
	}
	var (
		images  []string
		deleted bool
	)
	images, err = s.StorageImageServer().ResolveNames(image)
	if err != nil {
		if err == storage.ErrCannotParseImageID {
			images = append(images, image)
		} else {
			return nil, err
		}
	}
	for _, img := range images {
		err = s.StorageImageServer().UntagImage(s.ImageContext(), img)
		if err != nil {
			logrus.Debugf("error deleting image %s: %v", img, err)
			continue
		}
		deleted = true
		break
	}
	if !deleted && err != nil {
		return nil, err
	}
	resp = &pb.RemoveImageResponse{}
	logrus.Debugf("RemoveImageResponse: %+v", resp)
	return resp, nil
}
