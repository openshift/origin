package v1

import (
	"fmt"
	"sort"

	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/conversion"

	oapi "github.com/openshift/origin/pkg/api"
	newer "github.com/openshift/origin/pkg/image/api"
)

// The docker metadata must be cast to a version
func convert_api_Image_To_v1_Image(in *newer.Image, out *Image, s conversion.Scope) error {
	if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
		return err
	}

	out.DockerImageReference = in.DockerImageReference
	out.DockerImageManifest = in.DockerImageManifest

	version := in.DockerImageMetadataVersion
	if len(version) == 0 {
		version = "1.0"
	}
	data, err := kapi.Scheme.EncodeToVersion(&in.DockerImageMetadata, version)
	if err != nil {
		return err
	}
	out.DockerImageMetadata.RawJSON = data
	out.DockerImageMetadataVersion = version

	out.Finalizers = make([]kapiv1.FinalizerName, 0, 1)
	if err := s.Convert(&in.Finalizers, &out.Finalizers, 0); err != nil {
		return err
	}
	return s.Convert(&in.Status, &out.Status, 0)
}

func convert_v1_Image_To_api_Image(in *Image, out *newer.Image, s conversion.Scope) error {
	if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
		return err
	}

	out.DockerImageReference = in.DockerImageReference
	out.DockerImageManifest = in.DockerImageManifest

	version := in.DockerImageMetadataVersion
	if len(version) == 0 {
		version = "1.0"
	}
	if len(in.DockerImageMetadata.RawJSON) > 0 {
		// TODO: add a way to default the expected kind and version of an object if not set
		obj, err := kapi.Scheme.New(version, "DockerImage")
		if err != nil {
			return err
		}
		if err := kapi.Scheme.DecodeInto(in.DockerImageMetadata.RawJSON, obj); err != nil {
			return err
		}
		if err := s.Convert(obj, &out.DockerImageMetadata, 0); err != nil {
			return err
		}
	}
	out.DockerImageMetadataVersion = version

	out.Finalizers = make([]kapi.FinalizerName, 0, 1)
	if err := s.Convert(&in.Finalizers, &out.Finalizers, 0); err != nil {
		return err
	}

	if err := s.Convert(&in.Status, &out.Status, 0); err != nil {
		return err
	}

	if out.DeletionTimestamp == nil {
		// make sure that finalizers contain origin
		hasOriginFinalizer := false
		for _, finalizer := range out.Finalizers {
			if finalizer == oapi.FinalizerOrigin {
				hasOriginFinalizer = true
				break
			}
		}
		if !hasOriginFinalizer {
			out.Finalizers = append(out.Finalizers, oapi.FinalizerOrigin)
		}
		// phase need to match DeletionTimestamp
		out.Status.Phase = newer.ImageAvailable
	} else {
		out.Status.Phase = newer.ImagePurging
	}
	return nil
}

func convert_api_ImageStatus_To_v1_ImageStatus(in *newer.ImageStatus, out *ImageStatus, s conversion.Scope) error {
	out.Phase = in.Phase
	return nil
}

func convert_v1_ImageStatus_To_api_ImageStatus(in *ImageStatus, out *newer.ImageStatus, s conversion.Scope) error {
	out.Phase = in.Phase
	if out.Phase == "" {
		out.Phase = newer.ImageAvailable
	}
	return nil
}

func convert_v1_ImageStreamSpec_To_api_ImageStreamSpec(in *ImageStreamSpec, out *newer.ImageStreamSpec, s conversion.Scope) error {
	out.DockerImageRepository = in.DockerImageRepository
	out.Tags = make(map[string]newer.TagReference)
	return s.Convert(&in.Tags, &out.Tags, 0)
}

func convert_api_ImageStreamSpec_To_v1_ImageStreamSpec(in *newer.ImageStreamSpec, out *ImageStreamSpec, s conversion.Scope) error {
	out.DockerImageRepository = in.DockerImageRepository
	if len(in.DockerImageRepository) > 0 {
		// ensure that stored image references have no tag or ID, which was possible from 1.0.0 until 1.0.7
		if ref, err := newer.ParseDockerImageReference(in.DockerImageRepository); err == nil {
			if len(ref.Tag) > 0 || len(ref.ID) > 0 {
				ref.Tag, ref.ID = "", ""
				out.DockerImageRepository = ref.Exact()
			}
		}
	}
	out.Tags = make([]NamedTagReference, 0, 0)
	return s.Convert(&in.Tags, &out.Tags, 0)
}

func convert_v1_ImageStreamStatus_To_api_ImageStreamStatus(in *ImageStreamStatus, out *newer.ImageStreamStatus, s conversion.Scope) error {
	out.DockerImageRepository = in.DockerImageRepository
	out.Tags = make(map[string]newer.TagEventList)
	return s.Convert(&in.Tags, &out.Tags, 0)
}

func convert_api_ImageStreamStatus_To_v1_ImageStreamStatus(in *newer.ImageStreamStatus, out *ImageStreamStatus, s conversion.Scope) error {
	out.DockerImageRepository = in.DockerImageRepository
	if len(in.DockerImageRepository) > 0 {
		// ensure that stored image references have no tag or ID, which was possible from 1.0.0 until 1.0.7
		if ref, err := newer.ParseDockerImageReference(in.DockerImageRepository); err == nil {
			if len(ref.Tag) > 0 || len(ref.ID) > 0 {
				ref.Tag, ref.ID = "", ""
				out.DockerImageRepository = ref.Exact()
			}
		}
	}
	out.Tags = make([]NamedTagEventList, 0, 0)
	return s.Convert(&in.Tags, &out.Tags, 0)
}

func convert_api_ImageStreamMapping_To_v1_ImageStreamMapping(in *newer.ImageStreamMapping, out *ImageStreamMapping, s conversion.Scope) error {
	return s.DefaultConvert(in, out, conversion.DestFromSource)
}

func convert_v1_ImageStreamMapping_To_api_ImageStreamMapping(in *ImageStreamMapping, out *newer.ImageStreamMapping, s conversion.Scope) error {
	return s.DefaultConvert(in, out, conversion.SourceToDest)
}

func convert_api_ImageStreamDeletion_To_v1_ImageStreamDeletion(in *newer.ImageStreamDeletion, out *ImageStreamDeletion, s conversion.Scope) error {
	return s.DefaultConvert(in, out, conversion.DestFromSource)
}

func convert_v1_ImageStreamDeletion_To_api_ImageStreamDeletion(in *ImageStreamDeletion, out *newer.ImageStreamDeletion, s conversion.Scope) error {
	return s.DefaultConvert(in, out, conversion.SourceToDest)
}

func init() {
	err := kapi.Scheme.AddConversionFuncs(
		func(in *[]NamedTagEventList, out *map[string]newer.TagEventList, s conversion.Scope) error {
			for _, curr := range *in {
				newTagEventList := newer.TagEventList{}
				if err := s.Convert(&curr.Items, &newTagEventList.Items, 0); err != nil {
					return err
				}
				(*out)[curr.Tag] = newTagEventList
			}

			return nil
		},
		func(in *map[string]newer.TagEventList, out *[]NamedTagEventList, s conversion.Scope) error {
			allKeys := make([]string, 0, len(*in))
			for key := range *in {
				allKeys = append(allKeys, key)
			}
			sort.Strings(allKeys)

			for _, key := range allKeys {
				newTagEventList := (*in)[key]
				oldTagEventList := &NamedTagEventList{Tag: key}
				if err := s.Convert(&newTagEventList.Items, &oldTagEventList.Items, 0); err != nil {
					return err
				}

				*out = append(*out, *oldTagEventList)
			}

			return nil
		},
		func(in *[]NamedTagReference, out *map[string]newer.TagReference, s conversion.Scope) error {
			for _, curr := range *in {
				r := newer.TagReference{
					Annotations: curr.Annotations,
				}
				if err := s.Convert(&curr.From, &r.From, 0); err != nil {
					return err
				}
				(*out)[curr.Name] = r
			}
			return nil
		},
		func(in *map[string]newer.TagReference, out *[]NamedTagReference, s conversion.Scope) error {
			allTags := make([]string, 0, len(*in))
			for tag := range *in {
				allTags = append(allTags, tag)
			}
			sort.Strings(allTags)

			for _, tag := range allTags {
				newTagReference := (*in)[tag]
				oldTagReference := NamedTagReference{
					Name:        tag,
					Annotations: newTagReference.Annotations,
				}
				if err := s.Convert(&newTagReference.From, &oldTagReference.From, 0); err != nil {
					return err
				}
				*out = append(*out, oldTagReference)
			}
			return nil
		},

		convert_api_Image_To_v1_Image,
		convert_v1_Image_To_api_Image,
		convert_api_ImageStatus_To_v1_ImageStatus,
		convert_v1_ImageStatus_To_api_ImageStatus,
		convert_v1_ImageStreamSpec_To_api_ImageStreamSpec,
		convert_api_ImageStreamSpec_To_v1_ImageStreamSpec,
		convert_v1_ImageStreamStatus_To_api_ImageStreamStatus,
		convert_api_ImageStreamStatus_To_v1_ImageStreamStatus,
		convert_api_ImageStreamMapping_To_v1_ImageStreamMapping,
		convert_v1_ImageStreamMapping_To_api_ImageStreamMapping,
		convert_api_ImageStreamDeletion_To_v1_ImageStreamDeletion,
		convert_v1_ImageStreamDeletion_To_api_ImageStreamDeletion,
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}

	err = kapi.Scheme.AddFieldLabelConversionFunc("v1", "ImageStream",
		func(label, value string) (string, string, error) {
			switch label {
			case "name":
				return "metadata.name", value, nil
			case "metadata.name", "spec.dockerImageRepository", "status.dockerImageRepository":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}
