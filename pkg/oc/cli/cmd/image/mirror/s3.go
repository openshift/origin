package mirror

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"
	godigest "github.com/opencontainers/go-digest"
)

type s3Driver struct {
	UserAgent string
	Region    string
	Creds     auth.CredentialStore
	CopyFrom  []string

	repositories map[string]*s3.S3
}

type s3CredentialStore struct {
	store     auth.CredentialStore
	url       *url.URL
	retrieved bool
}

func (s *s3CredentialStore) IsExpired() bool { return !s.retrieved }

func (s *s3CredentialStore) Retrieve() (credentials.Value, error) {
	s.retrieved = false
	accessKeyID, secretAccessKey := s.store.Basic(s.url)
	if len(accessKeyID) == 0 || len(secretAccessKey) == 0 {
		return credentials.Value{}, fmt.Errorf("no AWS credentials located for %s", s.url)
	}
	s.retrieved = true
	glog.V(4).Infof("found credentials for %s", s.url)
	return credentials.Value{
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		ProviderName:    "DockerCfg",
	}, nil
}

func (d *s3Driver) newObject(server *url.URL, region string, insecure bool, securityDomain *url.URL) (*s3.S3, error) {
	key := fmt.Sprintf("%s:%s:%t:%s", server, region, insecure, securityDomain)
	s3obj, ok := d.repositories[key]
	if ok {
		return s3obj, nil
	}

	awsConfig := aws.NewConfig()

	var creds *credentials.Credentials
	creds = credentials.NewChainCredentials([]credentials.Provider{
		&s3CredentialStore{store: d.Creds, url: securityDomain},
		&credentials.EnvProvider{},
	})

	awsConfig.WithS3ForcePathStyle(true)
	awsConfig.WithEndpoint(server.String())
	awsConfig.WithCredentials(creds)
	awsConfig.WithRegion(region)
	awsConfig.WithDisableSSL(insecure)
	if glog.V(6) {
		awsConfig.WithLogLevel(aws.LogDebug)
	}

	if d.UserAgent != "" {
		awsConfig.WithHTTPClient(&http.Client{
			Transport: transport.NewTransport(http.DefaultTransport, transport.NewHeaderRequestModifier(http.Header{http.CanonicalHeaderKey("User-Agent"): []string{d.UserAgent}})),
		})
	}
	s, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, err
	}
	s3obj = s3.New(s)
	if d.repositories == nil {
		d.repositories = make(map[string]*s3.S3)
	}
	d.repositories[key] = s3obj
	return s3obj, nil
}

func (d *s3Driver) Repository(ctx context.Context, server *url.URL, repoName string, insecure bool) (distribution.Repository, error) {
	parts := strings.SplitN(repoName, "/", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("you must pass a three segment repository name for s3 uploads, where the first segment is the region and the second segment is the bucket")
	}
	s3obj, err := d.newObject(server, parts[0], insecure, &url.URL{Scheme: server.Scheme, Host: server.Host, Path: "/" + repoName})
	if err != nil {
		return nil, err
	}
	named, err := reference.ParseNamed(parts[2])
	if err != nil {
		return nil, err
	}
	repo := &s3Repository{
		ctx:      ctx,
		s3:       s3obj,
		bucket:   parts[1],
		repoName: named,
		copyFrom: d.CopyFrom,
	}
	return repo, nil
}

type s3Repository struct {
	ctx      context.Context
	s3       *s3.S3
	bucket   string
	once     sync.Once
	initErr  error
	copyFrom []string

	repoName reference.Named
}

// Named returns the name of the repository.
func (r *s3Repository) Named() reference.Named {
	return r.repoName
}

// Manifests returns a reference to this repository's manifest service.
// with the supplied options applied.
func (r *s3Repository) Manifests(ctx context.Context, options ...distribution.ManifestServiceOption) (distribution.ManifestService, error) {
	return &s3ManifestService{r: r}, nil
}

// Blobs returns a reference to this repository's blob service.
func (r *s3Repository) Blobs(ctx context.Context) distribution.BlobStore {
	return &s3BlobStore{r: r}
}

// Tags returns a reference to this repositories tag service
func (r *s3Repository) Tags(ctx context.Context) distribution.TagService {
	return nil
}

func (r *s3Repository) attemptCopy(id string, bucket, key string) bool {
	if _, err := r.s3.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}); err == nil {
		return true
	}
	if len(id) == 0 {
		return false
	}
	for _, copyFrom := range r.copyFrom {
		var sourceKey string
		if strings.HasSuffix(copyFrom, "[store]") {
			sourceKey = strings.TrimSuffix(copyFrom, "[store]")
			d, err := godigest.Parse(id)
			if err != nil {
				glog.V(4).Infof("Object %q is not a valid digest, cannot perform [store] copy: %v", id, err)
				continue
			}
			sourceKey = fmt.Sprintf("%s%s/%s/%s/data", sourceKey, d.Algorithm().String(), d.Hex()[:2], d.Hex())
		} else {
			sourceKey = path.Join(copyFrom, id)
		}
		_, err := r.s3.CopyObject(&s3.CopyObjectInput{
			CopySource: aws.String(sourceKey),
			Bucket:     aws.String(bucket),
			Key:        aws.String(key),
		})
		if err == nil {
			glog.V(4).Infof("Copied existing object from %s to %s", sourceKey, key)
			return true
		}
		if a, ok := err.(awserr.Error); ok && a.Code() == "NoSuchKey" {
			glog.V(4).Infof("No existing object matches source %s", sourceKey)
			continue
		}
		glog.V(4).Infof("Unable to copy from %s to %s: %v", sourceKey, key, err)
	}
	return false
}

func (r *s3Repository) conditionalUpload(input *s3manager.UploadInput, id string) error {
	if r.attemptCopy(id, *input.Bucket, *input.Key) {
		return nil
	}
	_, err := s3manager.NewUploaderWithClient(r.s3).Upload(input)
	return err
}

func (r *s3Repository) init() error {
	r.once.Do(func() {
		r.initErr = r.conditionalUpload(&s3manager.UploadInput{
			Bucket:   aws.String(r.bucket),
			Metadata: map[string]*string{"X-Docker-Distribution-API-Version": aws.String("registry/2.0")},
			Body:     bytes.NewBufferString(""),
			Key:      aws.String("/v2/"),
		}, "")
	})
	return r.initErr
}

type noSeekReader struct {
	io.Reader
}

var _ io.ReadSeeker = noSeekReader{}

func (noSeekReader) Seek(offset int64, whence int) (int64, error) {
	return 0, fmt.Errorf("unable to seek to %d via %d", offset, whence)
}

type s3ManifestService struct {
	r *s3Repository
}

// Exists returns true if the manifest exists.
func (s *s3ManifestService) Exists(ctx context.Context, dgst godigest.Digest) (bool, error) {
	return false, fmt.Errorf("unimplemented")
}

// Get retrieves the manifest specified by the given digest
func (s *s3ManifestService) Get(ctx context.Context, dgst godigest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	return nil, fmt.Errorf("unimplemented")
}

// Put creates or updates the given manifest returning the manifest digest
func (s *s3ManifestService) Put(ctx context.Context, manifest distribution.Manifest, options ...distribution.ManifestServiceOption) (godigest.Digest, error) {
	if err := s.r.init(); err != nil {
		return "", err
	}
	mediaType, payload, err := manifest.Payload()
	if err != nil {
		return "", err
	}
	dgst := godigest.FromBytes(payload)
	blob := fmt.Sprintf("/v2/%s/blobs/%s", s.r.repoName, dgst)

	if err := s.r.conditionalUpload(&s3manager.UploadInput{
		Bucket:      aws.String(s.r.bucket),
		ContentType: aws.String(mediaType),
		Body:        bytes.NewBuffer(payload),
		Key:         aws.String(blob),
	}, dgst.String()); err != nil {
		return "", err
	}

	// set manifests
	tags := []string{dgst.String()}
	for _, option := range options {
		if opt, ok := option.(distribution.WithTagOption); ok {
			tags = append(tags, opt.Tag)
		}
	}
	for _, tag := range tags {
		if _, err := s.r.s3.CopyObject(&s3.CopyObjectInput{
			Bucket:      aws.String(s.r.bucket),
			ContentType: aws.String(mediaType),
			CopySource:  aws.String(path.Join(s.r.bucket, blob)),
			Key:         aws.String(fmt.Sprintf("/v2/%s/manifests/%s", s.r.repoName, tag)),
		}); err != nil {
			return "", err
		}
	}
	return dgst, nil
}

// Delete removes the manifest specified by the given digest. Deleting
// a manifest that doesn't exist will return ErrManifestNotFound
func (s *s3ManifestService) Delete(ctx context.Context, dgst godigest.Digest) error {
	return fmt.Errorf("unimplemented")
}

type s3BlobStore struct {
	r *s3Repository
}

func (s *s3BlobStore) Stat(ctx context.Context, dgst godigest.Digest) (distribution.Descriptor, error) {
	return distribution.Descriptor{}, fmt.Errorf("unimplemented")
}

func (s *s3BlobStore) Delete(ctx context.Context, dgst godigest.Digest) error {
	return fmt.Errorf("unimplemented")
}

func (s *s3BlobStore) Get(ctx context.Context, dgst godigest.Digest) ([]byte, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (s *s3BlobStore) Open(ctx context.Context, dgst godigest.Digest) (distribution.ReadSeekCloser, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (s *s3BlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, r *http.Request, dgst godigest.Digest) error {
	return fmt.Errorf("unimplemented")
}

func (s *s3BlobStore) Put(ctx context.Context, mediaType string, p []byte) (distribution.Descriptor, error) {
	if err := s.r.init(); err != nil {
		return distribution.Descriptor{}, err
	}
	d := godigest.FromBytes(p)
	if err := s.r.conditionalUpload(&s3manager.UploadInput{
		Bucket:      aws.String(s.r.bucket),
		ContentType: aws.String(mediaType),
		Body:        bytes.NewBuffer(p),
		Key:         aws.String(fmt.Sprintf("/v2/%s/blobs/%s", s.r.repoName, d)),
	}, d.String()); err != nil {
		return distribution.Descriptor{}, err
	}
	return distribution.Descriptor{MediaType: mediaType, Size: int64(len(p)), Digest: d}, nil
}

func (s *s3BlobStore) Create(ctx context.Context, options ...distribution.BlobCreateOption) (distribution.BlobWriter, error) {
	var opts distribution.CreateOptions
	for _, option := range options {
		err := option.Apply(&opts)
		if err != nil {
			return nil, err
		}
	}
	if opts.Mount.Stat == nil || len(opts.Mount.Stat.Digest) == 0 {
		return nil, fmt.Errorf("S3 target blob store requires blobs to have mount stats that include a digest")
	}
	d := opts.Mount.Stat.Digest

	// attempt to copy before returning a writer
	key := fmt.Sprintf("/v2/%s/blobs/%s", s.r.repoName, d)
	if s.r.attemptCopy(d.String(), s.r.bucket, key) {
		return nil, ErrAlreadyExists
	}

	return s.r.newWriter(key, d.String(), opts.Mount.Stat.Size), nil
}

func (s *s3BlobStore) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	return nil, fmt.Errorf("unimplemented")
}

// writer attempts to upload parts to S3 in a buffered fashion where the last
// part is at least as large as the chunksize, so the multipart upload could be
// cleanly resumed in the future. This is violated if Close is called after less
// than a full chunk is written.
type writer struct {
	driver    *s3Repository
	key       string
	uploadID  string
	closed    bool
	committed bool
	cancelled bool
	size      int64
	startedAt time.Time
}

func (d *s3Repository) newWriter(key, uploadID string, size int64) distribution.BlobWriter {
	return &writer{
		driver:   d,
		key:      key,
		uploadID: uploadID,
		size:     size,
	}
}

func (w *writer) ID() string {
	return w.uploadID
}

func (w *writer) StartedAt() time.Time {
	return w.startedAt
}

func (w *writer) ReadFrom(r io.Reader) (int64, error) {
	switch {
	case w.closed:
		return 0, fmt.Errorf("already closed")
	case w.committed:
		return 0, fmt.Errorf("already committed")
	case w.cancelled:
		return 0, fmt.Errorf("already cancelled")
	}
	if w.startedAt.IsZero() {
		w.startedAt = time.Now()
	}
	_, err := s3manager.NewUploaderWithClient(w.driver.s3).Upload(&s3manager.UploadInput{
		Bucket:      aws.String(w.driver.bucket),
		ContentType: aws.String("application/octet-stream"),
		Key:         aws.String(w.key),
		Body:        r,
	})
	if err != nil {
		return 0, err
	}
	return w.size, nil
}

func (w *writer) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("already closed")
}

func (w *writer) Size() int64 {
	return w.size
}

func (w *writer) Close() error {
	switch {
	case w.closed:
		return fmt.Errorf("already closed")
	}
	w.closed = true
	return nil
}

func (w *writer) Cancel(ctx context.Context) error {
	switch {
	case w.closed:
		return fmt.Errorf("already closed")
	case w.committed:
		return fmt.Errorf("already committed")
	}
	w.cancelled = true
	return nil
}

// TODO: verify uploaded descriptor matches
func (w *writer) Commit(ctx context.Context, descriptor distribution.Descriptor) (distribution.Descriptor, error) {
	desc := descriptor
	switch {
	case w.closed:
		return desc, fmt.Errorf("already closed")
	case w.committed:
		return desc, fmt.Errorf("already committed")
	case w.cancelled:
		return desc, fmt.Errorf("already cancelled")
	}
	w.committed = true
	return desc, nil
}
