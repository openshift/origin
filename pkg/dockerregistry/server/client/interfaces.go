package client

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"

	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	userapiv1 "github.com/openshift/origin/pkg/user/apis/user/v1"
	authapiv1 "k8s.io/kubernetes/pkg/apis/authorization/v1"
)

type UsersInterfacer interface {
	Users() UserInterface
}

type ImagesInterfacer interface {
	Images() ImageInterface
}

type ImageSignaturesInterfacer interface {
	ImageSignatures() ImageSignatureInterface
}

type ImageStreamImagesNamespacer interface {
	ImageStreamImages(namespace string) ImageStreamImageInterface
}

type ImageStreamsNamespacer interface {
	ImageStreams(namespace string) ImageStreamInterface
}

type ImageStreamMappingsNamespacer interface {
	ImageStreamMappings(namespace string) ImageStreamMappingInterface
}

type ImageStreamSecretsNamespacer interface {
	ImageStreamSecrets(namespace string) ImageStreamSecretInterface
}

type ImageStreamTagsNamespacer interface {
	ImageStreamTags(namespace string) ImageStreamTagInterface
}

type LimitRangesGetter interface {
	LimitRanges(namespace string) LimitRangeInterface
}

type LocalSubjectAccessReviewsNamespacer interface {
	LocalSubjectAccessReviews(namespace string) LocalSubjectAccessReviewInterface
}

type SubjectAccessReviews interface {
	SubjectAccessReviews() SubjectAccessReviewInterface
}

type ImageSignatureInterface interface {
	Create(signature *imageapiv1.ImageSignature) (*imageapiv1.ImageSignature, error)
}

type ImageStreamImageInterface interface {
	Get(name string, options metav1.GetOptions) (*imageapiv1.ImageStreamImage, error)
}

type UserInterface interface {
	Get(name string, options metav1.GetOptions) (*userapiv1.User, error)
}

type ImageInterface interface {
	Get(name string, options metav1.GetOptions) (*imageapiv1.Image, error)
	Update(image *imageapiv1.Image) (*imageapiv1.Image, error)
	List(opts metav1.ListOptions) (*imageapiv1.ImageList, error)
}

type ImageStreamInterface interface {
	Get(name string, options metav1.GetOptions) (*imageapiv1.ImageStream, error)
	Create(stream *imageapiv1.ImageStream) (*imageapiv1.ImageStream, error)
}

type ImageStreamMappingInterface interface {
	Create(mapping *imageapiv1.ImageStreamMapping) (*imageapiv1.ImageStreamMapping, error)
}

type ImageStreamSecretInterface interface {
	Secrets(name string, options metav1.ListOptions) (*kapi.SecretList, error)
}

type ImageStreamTagInterface interface {
	Delete(name string, options *metav1.DeleteOptions) error
}

type LimitRangeInterface interface {
	List(opts metav1.ListOptions) (*kapi.LimitRangeList, error)
}

type LocalSubjectAccessReviewInterface interface {
	Create(policy *authapiv1.LocalSubjectAccessReview) (*authapiv1.LocalSubjectAccessReview, error)
}

type SubjectAccessReviewInterface interface {
	Create(policy *authapiv1.SelfSubjectAccessReview) (*authapiv1.SelfSubjectAccessReview, error)
}
