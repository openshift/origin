package resourceread

import (
	storagev1 "k8s.io/api/storage/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	storageScheme = runtime.NewScheme()
	storageCodecs = serializer.NewCodecFactory(storageScheme)
)

func init() {
	utilruntime.Must(storagev1.AddToScheme(storageScheme))
	utilruntime.Must(storagev1beta1.AddToScheme(storageScheme))
}

func ReadStorageClassV1OrDie(objBytes []byte) *storagev1.StorageClass {
	requiredObj, err := runtime.Decode(storageCodecs.UniversalDecoder(storagev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*storagev1.StorageClass)
}

func ReadCSIDriverV1Beta1OrDie(objBytes []byte) *storagev1beta1.CSIDriver {
	requiredObj, err := runtime.Decode(storageCodecs.UniversalDecoder(storagev1beta1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*storagev1beta1.CSIDriver)
}

func ReadCSIDriverV1OrDie(objBytes []byte) *storagev1.CSIDriver {
	requiredObj, err := runtime.Decode(storageCodecs.UniversalDecoder(storagev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*storagev1.CSIDriver)
}
