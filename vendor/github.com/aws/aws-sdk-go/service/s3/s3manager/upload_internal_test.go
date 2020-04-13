// +build go1.7

package s3manager

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	random "math/rand"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/awstesting/unit"
	"github.com/aws/aws-sdk-go/internal/sdkio"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/internal/s3testing"
)

type recordedPartPool struct {
	outstanding int64
	*partPool
}

func (r *recordedPartPool) Get() []byte {
	atomic.AddInt64(&r.outstanding, 1)
	return r.partPool.Get()
}

func (r *recordedPartPool) Put(b []byte) {
	atomic.AddInt64(&r.outstanding, -1)
	r.partPool.Put(b)
}

func swapByteSlicePool(f func(partSize int64) byteSlicePool) func() {
	orig := newByteSlicePool

	newByteSlicePool = f

	return func() {
		newByteSlicePool = orig
	}
}

func TestUploadByteSlicePool(t *testing.T) {
	cases := map[string]struct {
		PartSize   int64
		FileSize   int64
		Operations []string
	}{
		"single part": {
			PartSize: sdkio.MebiByte * 5,
			FileSize: sdkio.MebiByte * 5,
			Operations: []string{
				"PutObject",
			},
		},
		"multi-part": {
			PartSize: sdkio.MebiByte * 5,
			FileSize: sdkio.MebiByte * 10,
			Operations: []string{
				"CreateMultipartUpload",
				"UploadPart",
				"CompleteMultipartUpload",
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			for _, operation := range tt.Operations {
				t.Run(operation, func(t *testing.T) {
					var p *recordedPartPool

					unswap := swapByteSlicePool(func(partSize int64) byteSlicePool {
						p = &recordedPartPool{
							partPool: newPartPool(partSize),
						}
						return p
					})
					defer unswap()

					sess := unit.Session.Copy()
					svc := s3.New(sess)
					svc.Handlers.Unmarshal.Clear()
					svc.Handlers.UnmarshalMeta.Clear()
					svc.Handlers.UnmarshalError.Clear()
					svc.Handlers.Send.Clear()
					svc.Handlers.Send.PushFront(func(r *request.Request) {
						if r.Body != nil {
							io.Copy(ioutil.Discard, r.Body)
						}

						if r.Operation.Name == operation {
							r.Retryable = aws.Bool(false)
							r.Error = fmt.Errorf("request error")
							r.HTTPResponse = &http.Response{
								StatusCode: 500,
								Body:       ioutil.NopCloser(bytes.NewReader([]byte{})),
							}
							return
						}

						r.HTTPResponse = &http.Response{
							StatusCode: 200,
							Body:       ioutil.NopCloser(bytes.NewReader([]byte{})),
						}

						switch data := r.Data.(type) {
						case *s3.CreateMultipartUploadOutput:
							data.UploadId = aws.String("UPLOAD-ID")
						case *s3.UploadPartOutput:
							data.ETag = aws.String(fmt.Sprintf("ETAG%d", random.Int()))
						case *s3.CompleteMultipartUploadOutput:
							data.Location = aws.String("https://location")
							data.VersionId = aws.String("VERSION-ID")
						case *s3.PutObjectOutput:
							data.VersionId = aws.String("VERSION-ID")
						}
					})

					uploader := NewUploaderWithClient(svc, func(u *Uploader) {
						u.Concurrency = 1
						u.PartSize = tt.PartSize
					})

					expected := s3testing.GetTestBytes(int(tt.FileSize))
					_, err := uploader.Upload(&UploadInput{
						Bucket: aws.String("bucket"),
						Key:    aws.String("key"),
						Body:   bytes.NewReader(expected),
					})
					if err == nil {
						t.Fatalf("expected error but got none")
					}

					if v := atomic.LoadInt64(&p.outstanding); v != 0 {
						t.Fatalf("expected zero outsnatding pool parts, got %d", v)
					}
				})
			}
		})
	}
}
