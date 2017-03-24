package server

import (
	"reflect"
	"testing"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"

	registrytest "github.com/openshift/origin/pkg/dockerregistry/testutil"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestTagGet(t *testing.T) {
	namespace := "user"
	repo := "app"
	tag := "latest"

	os, client := registrytest.NewFakeOpenShiftWithClient()

	testImage, err := registrytest.RegisterRandomImage(os, namespace, repo, tag)
	if err != nil {
		t.Fatal(err)
	}

	testcases := []struct {
		title                 string
		tagName               string
		tagValue              distribution.Descriptor
		expectedError         bool
		expectedNotFoundError bool
		pullthrough           bool
		imageManaged          bool
	}{
		{
			title:        "get valid tag from managed image",
			tagName:      tag,
			tagValue:     distribution.Descriptor{Digest: digest.Digest(testImage.Name)},
			pullthrough:  true,
			imageManaged: true,
		},
		{
			title:        "get valid tag from managed image without pullthrough",
			tagName:      tag,
			tagValue:     distribution.Descriptor{Digest: digest.Digest(testImage.Name)},
			pullthrough:  false,
			imageManaged: true,
		},
		{
			title:                 "get valid tag from unmanaged image without pullthrough",
			tagName:               tag,
			pullthrough:           false,
			imageManaged:          false,
			expectedNotFoundError: true,
		},
		{
			title:                 "get missing tag",
			tagName:               tag + "-no-found",
			pullthrough:           true,
			imageManaged:          true,
			expectedError:         true,
			expectedNotFoundError: true,
		},
	}

	for _, tc := range testcases {
		if tc.imageManaged {
			testImage.Annotations[imageapi.ManagedByOpenShiftAnnotation] = "true"
		} else {
			testImage.Annotations[imageapi.ManagedByOpenShiftAnnotation] = "false"
		}

		r := newTestRepository(t, namespace, repo, testRepositoryOptions{
			client:            client,
			enablePullThrough: tc.pullthrough,
		})

		ts := &tagService{
			TagService: newTestTagService(nil),
			repo:       r,
		}

		resultDesc, err := ts.Get(context.Background(), tc.tagName)

		switch err.(type) {
		case distribution.ErrTagUnknown:
			if !tc.expectedNotFoundError {
				t.Fatalf("[%s] unexpected error: %#+v", tc.title, err)
			}
		case nil:
			if tc.expectedError || tc.expectedNotFoundError {
				t.Fatalf("[%s] unexpected successful response", tc.title)
			}
		default:
			if tc.expectedError {
				break
			}
			t.Fatalf("[%s] unexpected error: %#+v", tc.title, err)
		}

		if resultDesc.Digest != tc.tagValue.Digest {
			t.Fatalf("[%s] unexpected result returned", tc.title)
		}
	}
}

func TestTagGetWithoutImageStream(t *testing.T) {
	namespace := "user"
	repo := "app"
	tag := "latest"

	_, client := registrytest.NewFakeOpenShiftWithClient()

	r := newTestRepository(t, namespace, repo, testRepositoryOptions{
		client: client,
	})

	ts := &tagService{
		TagService: newTestTagService(nil),
		repo:       r,
	}

	_, err := ts.Get(context.Background(), tag)
	if err == nil {
		t.Fatalf("error expected")
	}

	_, ok := err.(distribution.ErrRepositoryUnknown)
	if !ok {
		t.Fatalf("unexpected error: %#+v", err)
	}
}

func TestTagCreation(t *testing.T) {
	namespace := "user"
	repo := "app"
	tag := "latest"

	os, client := registrytest.NewFakeOpenShiftWithClient()

	testImage, err := registrytest.RegisterRandomImage(os, namespace, repo, tag)
	if err != nil {
		t.Fatal(err)
	}

	testcases := []struct {
		title         string
		tagName       string
		tagValue      distribution.Descriptor
		expectedError bool
		pullthrough   bool
		imageManaged  bool
	}{
		{
			title:        "create tag on managed image with pullthrough",
			tagName:      tag + "-new",
			tagValue:     distribution.Descriptor{Digest: digest.Digest(testImage.Name)},
			pullthrough:  true,
			imageManaged: true,
		},
		{
			title:         "create tag on unmanaged image without pullthrough",
			tagName:       tag + "-new",
			tagValue:      distribution.Descriptor{Digest: digest.Digest(testImage.Name)},
			expectedError: true,
		},
		{
			title:         "create tag on missing image",
			tagName:       tag + "-new",
			tagValue:      distribution.Descriptor{Digest: digest.Digest(etcdDigest)},
			expectedError: true,
		},
	}

	for _, tc := range testcases {
		if tc.imageManaged {
			testImage.Annotations[imageapi.ManagedByOpenShiftAnnotation] = "true"
		} else {
			testImage.Annotations[imageapi.ManagedByOpenShiftAnnotation] = "false"
		}

		r := newTestRepository(t, namespace, repo, testRepositoryOptions{
			client:            client,
			enablePullThrough: tc.pullthrough,
		})

		ts := &tagService{
			TagService: newTestTagService(nil),
			repo:       r,
		}

		err := ts.Tag(context.Background(), tc.tagName, tc.tagValue)
		if tc.expectedError {
			if err == nil {
				t.Fatalf("[%s] error expected", tc.title)
			}
			continue
		}

		_, err = ts.Get(context.Background(), tc.tagName)
		if err == nil {
			t.Fatalf("[%s] error expected", tc.title)
		}
	}
}

func TestTagCreationWithoutImageStream(t *testing.T) {
	namespace := "user"
	repo := "app"
	tag := "latest"

	os, client := registrytest.NewFakeOpenShiftWithClient()

	anotherImage, err := registrytest.RegisterRandomImage(os, namespace, repo+"-another", tag)
	if err != nil {
		t.Fatal(err)
	}

	r := newTestRepository(t, namespace, repo, testRepositoryOptions{
		client: client,
	})

	ts := &tagService{
		TagService: newTestTagService(nil),
		repo:       r,
	}

	err = ts.Tag(context.Background(), tag, distribution.Descriptor{
		Digest: digest.Digest(anotherImage.Name),
	})
	if err == nil {
		t.Fatalf("error expected")
	}

	_, ok := err.(distribution.ErrRepositoryUnknown)
	if !ok {
		t.Fatalf("unexpected error: %#+v", err)
	}
}

func TestTagDeletion(t *testing.T) {
	namespace := "user"
	repo := "app"
	tag := "latest"

	os, client := registrytest.NewFakeOpenShiftWithClient()

	testImage, err := registrytest.RegisterRandomImage(os, namespace, repo, tag)
	if err != nil {
		t.Fatal(err)
	}

	testcases := []struct {
		title                 string
		tagName               string
		tagValue              distribution.Descriptor
		expectedError         bool
		expectedNotFoundError bool
		pullthrough           bool
		imageManaged          bool
	}{
		{
			title:        "delete tag from managed image with pullthrough",
			tagName:      tag,
			pullthrough:  true,
			imageManaged: true,
		},
		{
			title:        "delete tag from managed image without pullthrough",
			tagName:      tag,
			imageManaged: true,
		},
		{
			title:       "delete tag from unmanaged image with pullthrough",
			tagName:     tag,
			pullthrough: true,
		},
		{
			title:                 "delete tag from unmanaged image without pullthrough",
			tagName:               tag,
			expectedNotFoundError: true,
		},
		{
			title:                 "delete wrong tag",
			tagName:               tag + "-not-found",
			expectedNotFoundError: true,
		},
	}

	for _, tc := range testcases {
		if tc.imageManaged {
			testImage.Annotations[imageapi.ManagedByOpenShiftAnnotation] = "true"
		} else {
			testImage.Annotations[imageapi.ManagedByOpenShiftAnnotation] = "false"
		}

		r := newTestRepository(t, namespace, repo, testRepositoryOptions{
			client:            client,
			enablePullThrough: tc.pullthrough,
		})

		ts := &tagService{
			TagService: newTestTagService(nil),
			repo:       r,
		}

		err := ts.Untag(context.Background(), tc.tagName)

		switch err.(type) {
		case distribution.ErrTagUnknown:
			if !tc.expectedNotFoundError {
				t.Fatalf("[%s] unexpected error: %#+v", tc.title, err)
			}
		case nil:
			if tc.expectedError || tc.expectedNotFoundError {
				t.Fatalf("[%s] unexpected successful response", tc.title)
			}
		default:
			if tc.expectedError {
				break
			}
			t.Fatalf("[%s] unexpected error: %#+v", tc.title, err)
		}
	}
}

func TestTagDeletionWithoutImageStream(t *testing.T) {
	namespace := "user"
	repo := "app"
	tag := "latest"

	_, client := registrytest.NewFakeOpenShiftWithClient()

	r := newTestRepository(t, namespace, repo, testRepositoryOptions{
		client: client,
	})

	ts := &tagService{
		TagService: newTestTagService(nil),
		repo:       r,
	}

	err := ts.Untag(context.Background(), tag)
	if err == nil {
		t.Fatalf("error expected")
	}

	_, ok := err.(distribution.ErrRepositoryUnknown)
	if !ok {
		t.Fatalf("unexpected error: %#+v", err)
	}
}

func TestTagGetAll(t *testing.T) {
	namespace := "user"
	repo := "app"
	tag := "latest"

	os, client := registrytest.NewFakeOpenShiftWithClient()

	testImage, err := registrytest.RegisterRandomImage(os, namespace, repo, tag)
	if err != nil {
		t.Fatal(err)
	}

	testcases := []struct {
		title         string
		expectResult  []string
		expectedError bool
		pullthrough   bool
		imageManaged  bool
	}{
		{
			title:        "get all tags with pullthrough",
			expectResult: []string{tag},
			pullthrough:  true,
			imageManaged: true,
		},
		{
			title:        "get all tags without pullthrough",
			expectResult: []string{tag},
			imageManaged: true,
		},
		{
			title:        "get all tags from unmanaged image without pullthrough",
			expectResult: []string{},
		},
	}

	for _, tc := range testcases {
		if tc.imageManaged {
			testImage.Annotations[imageapi.ManagedByOpenShiftAnnotation] = "true"
		} else {
			testImage.Annotations[imageapi.ManagedByOpenShiftAnnotation] = "false"
		}

		r := newTestRepository(t, namespace, repo, testRepositoryOptions{
			client:            client,
			enablePullThrough: tc.pullthrough,
		})

		ts := &tagService{
			TagService: newTestTagService(nil),
			repo:       r,
		}

		result, err := ts.All(context.Background())

		if err != nil && !tc.expectedError {
			t.Fatalf("[%s] unexpected error: %#+v", tc.title, err)
		}

		if !reflect.DeepEqual(result, tc.expectResult) {
			t.Fatalf("[%s] unexpected result: %#+v", tc.title, result)
		}
	}
}

func TestTagGetAllWithoutImageStream(t *testing.T) {
	namespace := "user"
	repo := "app"

	_, client := registrytest.NewFakeOpenShiftWithClient()

	r := newTestRepository(t, namespace, repo, testRepositoryOptions{
		client: client,
	})

	ts := &tagService{
		TagService: newTestTagService(nil),
		repo:       r,
	}

	_, err := ts.All(context.Background())
	if err == nil {
		t.Fatalf("error expected")
	}

	_, ok := err.(distribution.ErrRepositoryUnknown)
	if !ok {
		t.Fatalf("unexpected error: %#+v", err)
	}
}

func TestTagLookup(t *testing.T) {
	namespace := "user"
	repo := "app"
	tag := "latest"

	os, client := registrytest.NewFakeOpenShiftWithClient()

	testImage, err := registrytest.RegisterRandomImage(os, namespace, repo, tag)
	if err != nil {
		t.Fatal(err)
	}

	testcases := []struct {
		title         string
		tagValue      distribution.Descriptor
		expectResult  []string
		expectedError bool
		pullthrough   bool
		imageManaged  bool
	}{
		{
			title:        "lookup tags with pullthrough",
			tagValue:     distribution.Descriptor{Digest: digest.Digest(testImage.Name)},
			expectResult: []string{tag},
			pullthrough:  true,
			imageManaged: true,
		},
		{
			title:        "lookup tags without pullthrough",
			tagValue:     distribution.Descriptor{Digest: digest.Digest(testImage.Name)},
			expectResult: []string{tag},
			imageManaged: true,
		},
		{
			title:        "lookup tags by missing digest",
			tagValue:     distribution.Descriptor{Digest: digest.Digest(etcdDigest)},
			expectResult: []string{},
			pullthrough:  true,
			imageManaged: true,
		},
		{
			title:        "lookup tags in unmanaged images without pullthrough",
			tagValue:     distribution.Descriptor{Digest: digest.Digest(testImage.Name)},
			expectResult: []string{},
		},
	}

	for _, tc := range testcases {
		if tc.imageManaged {
			testImage.Annotations[imageapi.ManagedByOpenShiftAnnotation] = "true"
		} else {
			testImage.Annotations[imageapi.ManagedByOpenShiftAnnotation] = "false"
		}

		r := newTestRepository(t, namespace, repo, testRepositoryOptions{
			client:            client,
			enablePullThrough: tc.pullthrough,
		})

		ts := &tagService{
			TagService: newTestTagService(nil),
			repo:       r,
		}

		result, err := ts.Lookup(context.Background(), tc.tagValue)

		if err != nil {
			if !tc.expectedError {
				t.Fatalf("[%s] unexpected error: %#+v", tc.title, err)
			}
			continue
		} else {
			if tc.expectedError {
				t.Fatalf("[%s] error expected", tc.title)
			}
		}

		if !reflect.DeepEqual(result, tc.expectResult) {
			t.Fatalf("[%s] unexpected result: %#+v", tc.title, result)
		}
	}
}

func TestTagLookupWithoutImageStream(t *testing.T) {
	namespace := "user"
	repo := "app"
	tag := "latest"

	os, client := registrytest.NewFakeOpenShiftWithClient()

	anotherImage, err := registrytest.RegisterRandomImage(os, namespace, repo+"-another", tag)
	if err != nil {
		t.Fatal(err)
	}

	r := newTestRepository(t, namespace, repo, testRepositoryOptions{
		client: client,
	})

	ts := &tagService{
		TagService: newTestTagService(nil),
		repo:       r,
	}

	_, err = ts.Lookup(context.Background(), distribution.Descriptor{
		Digest: digest.Digest(anotherImage.Name),
	})
	if err == nil {
		t.Fatalf("error expected")
	}

	_, ok := err.(distribution.ErrRepositoryUnknown)
	if !ok {
		t.Fatalf("unexpected error: %#+v", err)
	}
}

type testTagService struct {
	data  map[string]distribution.Descriptor
	calls map[string]int
}

func newTestTagService(data map[string]distribution.Descriptor) *testTagService {
	b := make(map[string]distribution.Descriptor)
	for d, content := range data {
		b[d] = content
	}
	return &testTagService{
		data:  b,
		calls: make(map[string]int),
	}
}

func (t *testTagService) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
	t.calls["Get"]++
	desc, exists := t.data[tag]
	if !exists {
		return distribution.Descriptor{}, distribution.ErrTagUnknown{Tag: tag}
	}
	return desc, nil
}

func (t *testTagService) Tag(ctx context.Context, tag string, desc distribution.Descriptor) error {
	t.calls["Tag"]++
	t.data[tag] = desc
	return nil
}

func (t *testTagService) Untag(ctx context.Context, tag string) error {
	t.calls["Untag"]++
	_, exists := t.data[tag]
	if !exists {
		return distribution.ErrTagUnknown{Tag: tag}
	}
	delete(t.data, tag)
	return nil
}

func (t *testTagService) All(ctx context.Context) (tags []string, err error) {
	t.calls["All"]++
	for tag := range t.data {
		tags = append(tags, tag)
	}
	return
}

func (t *testTagService) Lookup(ctx context.Context, desc distribution.Descriptor) (tags []string, err error) {
	t.calls["Lookup"]++
	for tag := range t.data {
		if t.data[tag].Digest == desc.Digest {
			tags = append(tags, tag)
		}
	}
	return
}
