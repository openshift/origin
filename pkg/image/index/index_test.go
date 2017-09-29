package index

import (
	"testing"

	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	ktestclient "k8s.io/client-go/testing"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

func addImage(name string, layers []string) imageapi.Image {
	image := imageapi.Image{}
	image.Name = name
	image.DockerImageLayers = []imageapi.ImageLayer{}
	for _, l := range layers {
		image.DockerImageLayers = append(image.DockerImageLayers, imageapi.ImageLayer{Name: l, LayerSize: int64(100)})
	}
	return image
}

func simpleImageChain() []imageapi.Image {
	result := []imageapi.Image{}
	result = append(result, addImage("base", []string{
		"sha256:9c3823d8bc18257d5552703135c2222d2c083a296064ed2b151eaa21e819770e",
		"sha256:8f045733649f36ff037148858463355dca8f224da31835baf153b391eb915adb",
		"sha256:2da06dc69380add7862cf69f0b66cb9a2873023e901e7e024e2107cef3351b6e",
	}))
	result = append(result, addImage("s2i-base", []string{
		"sha256:9c3823d8bc18257d5552703135c2222d2c083a296064ed2b151eaa21e819770e",
		"sha256:8f045733649f36ff037148858463355dca8f224da31835baf153b391eb915adb",
		"sha256:2da06dc69380add7862cf69f0b66cb9a2873023e901e7e024e2107cef3351b6e",
		"sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef",
	}))
	result = append(result, addImage("s2i-python", []string{
		"sha256:9c3823d8bc18257d5552703135c2222d2c083a296064ed2b151eaa21e819770e",
		"sha256:8f045733649f36ff037148858463355dca8f224da31835baf153b391eb915adb",
		"sha256:2da06dc69380add7862cf69f0b66cb9a2873023e901e7e024e2107cef3351b6e",
		"sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef",
		"sha256:d9b34ea2353dc308b7e9b1d7c8a40344691dc0801332004fef8960a026f869ed",
	}))
	result = append(result, addImage("app", []string{
		"sha256:9c3823d8bc18257d5552703135c2222d2c083a296064ed2b151eaa21e819770e",
		"sha256:8f045733649f36ff037148858463355dca8f224da31835baf153b391eb915adb",
		"sha256:2da06dc69380add7862cf69f0b66cb9a2873023e901e7e024e2107cef3351b6e",
		"sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef",
		"sha256:d9b34ea2353dc308b7e9b1d7c8a40344691dc0801332004fef8960a026f869ed",
		"sha256:11111112353dc308b7e9b1d7c8a40344691dc0801332004fef8960a026f869ed",
	}))
	return result
}

func getImage(list []imageapi.Image, name string) *imageapi.Image {
	for _, i := range list {
		if i.Name == name {
			return &i
		}
	}
	return nil
}

func TestSimpleImageIndexAncestors(t *testing.T) {
	fake := &testclient.Fake{}
	fake.AddReactor("list", "images", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		result := imageapi.ImageList{Items: simpleImageChain()}
		return true, &result, nil
	})
	fake.AddWatchReactor("images", ktestclient.DefaultWatchReactor(watch.NewFake(), nil))

	index := NewImageIndex(fake.Images(), make(chan struct{}))

	index.WaitForSyncedStores()
	simpleChain := simpleImageChain()

	descendantsTable := map[string][]string{
		"base":       []string{},
		"s2i-python": []string{"s2i-base", "base"},
		"app":        []string{"s2i-python", "s2i-base", "base"},
	}

	for imageName, expect := range descendantsTable {
		images, err := index.Ancestors(getImage(simpleChain, imageName))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(expect) != len(images) {
			imageNameList := []string{}
			for _, i := range images {
				imageNameList = append(imageNameList, i.Name)
			}
			t.Errorf("[%s] expected %v, got %#+v", imageName, expect, imageNameList)
			continue
		}
		for _, e := range expect {
			found := false
			for _, got := range images {
				if got.Name == e {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected %q in the descendants", e)
			}
		}
	}
}

func TestSimpleImageIndexDescendants(t *testing.T) {
	fake := &testclient.Fake{}
	fake.AddReactor("list", "images", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		result := imageapi.ImageList{Items: simpleImageChain()}
		return true, &result, nil
	})
	fake.AddWatchReactor("images", ktestclient.DefaultWatchReactor(watch.NewFake(), nil))

	index := NewImageIndex(fake.Images(), make(chan struct{}))

	index.WaitForSyncedStores()
	simpleChain := simpleImageChain()

	ancestorTable := map[string][]string{
		"base":       []string{"s2i-base", "s2i-python", "app"},
		"s2i-python": []string{"app"},
		"app":        []string{},
	}

	for imageName, expect := range ancestorTable {
		images, err := index.Descendants(getImage(simpleChain, imageName))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(expect) != len(images) {
			t.Errorf("expected %v, got %#+v", expect, images)
			continue
		}
		for _, e := range expect {
			found := false
			for _, got := range images {
				if got.Name == e {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected %q in the ancestors", e)
			}
		}
	}
}
