package resourceread

import (
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	storageScheme = runtime.NewScheme()
	storageCodecs = serializer.NewCodecFactory(storageScheme)
)

func init() {
	if err := storagev1.AddToScheme(storageScheme); err != nil {
		panic(err)
	}
}

func ReadStorageClassV1OrDie(objBytes []byte) *storagev1.StorageClass {
	requiredObj, err := runtime.Decode(storageCodecs.UniversalDecoder(storagev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*storagev1.StorageClass)
}
