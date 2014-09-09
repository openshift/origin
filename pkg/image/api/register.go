package api

import "github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

func init() {
	runtime.AddKnownTypes("",
		Image{},
		ImageList{},
		ImageRepository{},
		ImageRepositoryList{},
		ImageRepositoryMapping{},
	)
}
