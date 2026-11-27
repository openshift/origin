package grpc_interop

import (
	"context"
	"io"
	"testing"

	"google.golang.org/grpc"
	testpb "google.golang.org/grpc/interop/grpc_testing"
	"google.golang.org/grpc/metadata"
)

func validHeader() metadata.MD {
	return metadata.MD{initialMetadataKey: []string{initialMetadataValue}}
}

func validTrailer() metadata.MD {
	return metadata.MD{trailingMetadataKey: []string{trailingMetadataValue}}
}

type fakeInteropClient struct {
	unaryHeader   metadata.MD
	unaryTrailer  metadata.MD
	streamHeader  metadata.MD
	streamTrailer metadata.MD
}

func (f *fakeInteropClient) UnaryCall(_ context.Context, _ *testpb.SimpleRequest, opts ...grpc.CallOption) (*testpb.SimpleResponse, error) {
	for _, opt := range opts {
		switch o := opt.(type) {
		case grpc.HeaderCallOption:
			*o.HeaderAddr = f.unaryHeader
		case grpc.TrailerCallOption:
			*o.TrailerAddr = f.unaryTrailer
		}
	}
	return &testpb.SimpleResponse{
		Payload: &testpb.Payload{Type: testpb.PayloadType_COMPRESSABLE, Body: make([]byte, 1)},
	}, nil
}

func (f *fakeInteropClient) FullDuplexCall(context.Context, ...grpc.CallOption) (grpc.BidiStreamingClient[testpb.StreamingOutputCallRequest, testpb.StreamingOutputCallResponse], error) {
	return &fakeBidiStream{header: f.streamHeader, trailer: f.streamTrailer}, nil
}

func (f *fakeInteropClient) EmptyCall(context.Context, *testpb.Empty, ...grpc.CallOption) (*testpb.Empty, error) {
	return nil, nil
}

func (f *fakeInteropClient) CacheableUnaryCall(context.Context, *testpb.SimpleRequest, ...grpc.CallOption) (*testpb.SimpleResponse, error) {
	return nil, nil
}

func (f *fakeInteropClient) StreamingOutputCall(context.Context, *testpb.StreamingOutputCallRequest, ...grpc.CallOption) (grpc.ServerStreamingClient[testpb.StreamingOutputCallResponse], error) {
	return nil, nil
}

func (f *fakeInteropClient) StreamingInputCall(context.Context, ...grpc.CallOption) (grpc.ClientStreamingClient[testpb.StreamingInputCallRequest, testpb.StreamingInputCallResponse], error) {
	return nil, nil
}

func (f *fakeInteropClient) HalfDuplexCall(context.Context, ...grpc.CallOption) (grpc.BidiStreamingClient[testpb.StreamingOutputCallRequest, testpb.StreamingOutputCallResponse], error) {
	return nil, nil
}

func (f *fakeInteropClient) UnimplementedCall(context.Context, *testpb.Empty, ...grpc.CallOption) (*testpb.Empty, error) {
	return nil, nil
}

type fakeBidiStream struct {
	header       metadata.MD
	trailer      metadata.MD
	recvReturned bool
}

func (s *fakeBidiStream) Send(*testpb.StreamingOutputCallRequest) error { return nil }

func (s *fakeBidiStream) Recv() (*testpb.StreamingOutputCallResponse, error) {
	if s.recvReturned {
		return nil, io.EOF
	}
	s.recvReturned = true
	return &testpb.StreamingOutputCallResponse{
		Payload: &testpb.Payload{Type: testpb.PayloadType_COMPRESSABLE, Body: make([]byte, 1)},
	}, nil
}

func (s *fakeBidiStream) Header() (metadata.MD, error) { return s.header, nil }
func (s *fakeBidiStream) Trailer() metadata.MD         { return s.trailer }
func (s *fakeBidiStream) CloseSend() error             { return nil }
func (s *fakeBidiStream) Context() context.Context     { return context.Background() }
func (s *fakeBidiStream) SendMsg(any) error            { return nil }
func (s *fakeBidiStream) RecvMsg(any) error            { return nil }

func TestDoCustomMetadata(t *testing.T) {
	tests := []struct {
		name   string
		client *fakeInteropClient
	}{
		{
			name: "correct metadata",
			client: &fakeInteropClient{
				unaryHeader: validHeader(), unaryTrailer: validTrailer(),
				streamHeader: validHeader(), streamTrailer: validTrailer(),
			},
		},
		{
			name: "missing unary header",
			client: &fakeInteropClient{
				unaryHeader: metadata.MD{}, unaryTrailer: validTrailer(),
				streamHeader: validHeader(), streamTrailer: validTrailer(),
			},
		},
		{
			name: "wrong unary trailer value",
			client: &fakeInteropClient{
				unaryHeader: validHeader(), unaryTrailer: metadata.MD{trailingMetadataKey: []string{"wrong"}},
				streamHeader: validHeader(), streamTrailer: validTrailer(),
			},
		},
		{
			name: "missing stream header",
			client: &fakeInteropClient{
				unaryHeader: validHeader(), unaryTrailer: validTrailer(),
				streamHeader: metadata.MD{}, streamTrailer: validTrailer(),
			},
		},
		{
			name: "missing stream trailer",
			client: &fakeInteropClient{
				unaryHeader: validHeader(), unaryTrailer: validTrailer(),
				streamHeader: validHeader(), streamTrailer: metadata.MD{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := DoCustomMetadata(tt.client)
			if tt.name == "correct metadata" {
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error for invalid metadata, got nil")
			}
		})
	}
}
