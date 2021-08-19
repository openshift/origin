package resourceread

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	migrationv1alpha1 "sigs.k8s.io/kube-storage-version-migrator/pkg/apis/migration/v1alpha1"
)

var (
	migrationScheme = runtime.NewScheme()
	migrationCodecs = serializer.NewCodecFactory(migrationScheme)
)

func init() {
	if err := migrationv1alpha1.AddToScheme(migrationScheme); err != nil {
		panic(err)
	}
}

func ReadStorageVersionMigrationV1Alpha1OrDie(objBytes []byte) *migrationv1alpha1.StorageVersionMigration {
	requiredObj, err := runtime.Decode(migrationCodecs.UniversalDecoder(migrationv1alpha1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*migrationv1alpha1.StorageVersionMigration)
}
