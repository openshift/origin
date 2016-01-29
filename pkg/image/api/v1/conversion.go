package v1

import (
	"sort"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	v1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"

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

	gvString := in.DockerImageMetadataVersion
	if len(gvString) == 0 {
		gvString = "1.0"
	}
	if !strings.Contains(gvString, "/") {
		gvString = "/" + gvString
	}

	version, err := unversioned.ParseGroupVersion(gvString)
	if err != nil {
		return err
	}
	data, err := runtime.Encode(api.Codecs.LegacyCodec(version), &in.DockerImageMetadata, version)
	if err != nil {
		return err
	}
	out.DockerImageMetadata.RawJSON = data
	out.DockerImageMetadataVersion = version.Version

	if in.DockerImageLayers != nil {
		out.DockerImageLayers = make([]ImageLayer, len(in.DockerImageLayers))
		for i := range in.DockerImageLayers {
			if err := s.Convert(&in.DockerImageLayers[i], &out.DockerImageLayers[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.DockerImageLayers = nil
	}

	return nil
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
		obj, err := api.Scheme.New(unversioned.GroupVersionKind{Version: version, Kind: "DockerImage"})
		if err != nil {
			return err
		}
		if err := runtime.DecodeInto(api.Codecs.UniversalDecoder(), in.DockerImageMetadata.RawJSON, obj); err != nil {
			return err
		}
		if err := s.Convert(obj, &out.DockerImageMetadata, 0); err != nil {
			return err
		}
	}
	out.DockerImageMetadataVersion = version

	if in.DockerImageLayers != nil {
		out.DockerImageLayers = make([]newer.ImageLayer, len(in.DockerImageLayers))
		for i := range in.DockerImageLayers {
			if err := s.Convert(&in.DockerImageLayers[i], &out.DockerImageLayers[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.DockerImageLayers = nil
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
	out.Tags = make([]TagReference, 0, 0)
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

func addConversionFuncs(scheme *runtime.Scheme) {
	err := scheme.AddDefaultingFuncs(
		func(obj *ImageImportSpec) {
			if obj.To == nil {
				if ref, err := newer.ParseDockerImageReference(obj.From.Name); err == nil {
					if len(ref.Tag) > 0 {
						obj.To = &v1.LocalObjectReference{Name: ref.Tag}
					}
				}
			}
		})
	if err != nil {
		// If one of the default functions is malformed, detect it immediately.
		panic(err)
	}
	err = scheme.AddConversionFuncs(
		func(in *[]NamedTagEventList, out *map[string]newer.TagEventList, s conversion.Scope) error {
			for _, curr := range *in {
				newTagEventList := newer.TagEventList{}
				if err := s.Convert(&curr.Conditions, &newTagEventList.Conditions, 0); err != nil {
					return err
				}
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
				if err := s.Convert(&newTagEventList.Conditions, &oldTagEventList.Conditions, 0); err != nil {
					return err
				}
				if err := s.Convert(&newTagEventList.Items, &oldTagEventList.Items, 0); err != nil {
					return err
				}

				*out = append(*out, *oldTagEventList)
			}

			return nil
		},
		func(in *[]TagReference, out *map[string]newer.TagReference, s conversion.Scope) error {
			for _, curr := range *in {
				r := newer.TagReference{}
				if err := s.Convert(&curr, &r, 0); err != nil {
					return err
				}
				(*out)[curr.Name] = r
			}
			return nil
		},
		func(in *map[string]newer.TagReference, out *[]TagReference, s conversion.Scope) error {
			allTags := make([]string, 0, len(*in))
			for tag := range *in {
				allTags = append(allTags, tag)
			}
			sort.Strings(allTags)

			for _, tag := range allTags {
				newTagReference := (*in)[tag]
				oldTagReference := TagReference{}
				if err := s.Convert(&newTagReference, &oldTagReference, 0); err != nil {
					return err
				}
				oldTagReference.Name = tag
				*out = append(*out, oldTagReference)
			}
			return nil
		},

		convert_api_Image_To_v1_Image,
		convert_v1_Image_To_api_Image,
		convert_v1_ImageStreamSpec_To_api_ImageStreamSpec,
		convert_api_ImageStreamSpec_To_v1_ImageStreamSpec,
		convert_v1_ImageStreamStatus_To_api_ImageStreamStatus,
		convert_api_ImageStreamStatus_To_v1_ImageStreamStatus,
		convert_api_ImageStreamMapping_To_v1_ImageStreamMapping,
		convert_v1_ImageStreamMapping_To_api_ImageStreamMapping,
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "Image",
		oapi.GetFieldLabelConversionFunc(newer.ImageToSelectableFields(&newer.Image{}), nil),
	); err != nil {
		panic(err)
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "ImageStream",
		oapi.GetFieldLabelConversionFunc(newer.ImageStreamToSelectableFields(&newer.ImageStream{}), map[string]string{"name": "metadata.name"}),
	); err != nil {
		panic(err)
	}
}
