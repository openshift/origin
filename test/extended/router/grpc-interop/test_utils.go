// This is a copy of:
//
//    origin(@4.5)/vendor/google.golang.org/grpc/interop/test_utils.go.
//
// Any function that called Fatal now returns an error.
// Unused functions have been removed.

/*
 *
 * Copyright 2014 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package grpc_interop

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	testpb "google.golang.org/grpc/interop/grpc_testing"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	reqSizes            = []int{27182, 8, 1828, 45904}
	respSizes           = []int{31415, 9, 2653, 58979}
	largeReqSize        = 271828
	largeRespSize       = 314159
	initialMetadataKey  = "x-grpc-test-echo-initial"
	trailingMetadataKey = "x-grpc-test-echo-trailing-bin"
)

// ClientNewPayload returns a payload of the given type and size.
func ClientNewPayload(t testpb.PayloadType, size int) (*testpb.Payload, error) {
	if size < 0 {
		return nil, fmt.Errorf("Requested a response with invalid length %d", size)
	}
	body := make([]byte, size)
	switch t {
	case testpb.PayloadType_COMPRESSABLE:
	default:
		return nil, fmt.Errorf("Unsupported payload type: %d", t)
	}
	return &testpb.Payload{
		Type: t,
		Body: body,
	}, nil
}

// DoEmptyUnaryCall performs a unary RPC with empty request and response messages.
func DoEmptyUnaryCall(tc testpb.TestServiceClient, args ...grpc.CallOption) error {
	reply, err := tc.EmptyCall(context.Background(), &testpb.Empty{}, args...)
	if err != nil {
		return fmt.Errorf("/TestService/EmptyCall RPC failed: %v", err)
	}
	if !proto.Equal(&testpb.Empty{}, reply) {
		return fmt.Errorf("/TestService/EmptyCall receives %v, want %v", reply, testpb.Empty{})
	}
	return nil
}

// DoLargeUnaryCall performs a unary RPC with large payload in the request and response.
func DoLargeUnaryCall(tc testpb.TestServiceClient, args ...grpc.CallOption) error {
	pl, err := ClientNewPayload(testpb.PayloadType_COMPRESSABLE, largeReqSize)
	if err != nil {
		return err
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: int32(largeRespSize),
		Payload:      pl,
	}
	reply, err := tc.UnaryCall(context.Background(), req, args...)
	if err != nil {
		return fmt.Errorf("/TestService/UnaryCall RPC failed: %v", err)
	}
	t := reply.GetPayload().GetType()
	s := len(reply.GetPayload().GetBody())
	if t != testpb.PayloadType_COMPRESSABLE || s != largeRespSize {
		return fmt.Errorf("Got the reply with type %d len %d; want %d, %d", t, s, testpb.PayloadType_COMPRESSABLE, largeRespSize)
	}
	return nil
}

// DoClientStreaming performs a client streaming RPC.
func DoClientStreaming(tc testpb.TestServiceClient, args ...grpc.CallOption) error {
	stream, err := tc.StreamingInputCall(context.Background(), args...)
	if err != nil {
		return fmt.Errorf("%v.StreamingInputCall(_) = _, %v", tc, err)
	}
	var sum int
	for _, s := range reqSizes {
		pl, err := ClientNewPayload(testpb.PayloadType_COMPRESSABLE, s)
		if err != nil {
			return err
		}
		req := &testpb.StreamingInputCallRequest{
			Payload: pl,
		}
		if err := stream.Send(req); err != nil {
			return fmt.Errorf("%v has error %v while sending %v", stream, err, req)
		}
		sum += s
	}
	reply, err := stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("%v.CloseAndRecv() got error %v, want %v", stream, err, nil)
	}
	if reply.GetAggregatedPayloadSize() != int32(sum) {
		return fmt.Errorf("%v.CloseAndRecv().GetAggregatePayloadSize() = %v; want %v", stream, reply.GetAggregatedPayloadSize(), sum)
	}
	return nil
}

// DoServerStreaming performs a server streaming RPC.
func DoServerStreaming(tc testpb.TestServiceClient, args ...grpc.CallOption) error {
	respParam := make([]*testpb.ResponseParameters, len(respSizes))
	for i, s := range respSizes {
		respParam[i] = &testpb.ResponseParameters{
			Size: int32(s),
		}
	}
	req := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
	}
	stream, err := tc.StreamingOutputCall(context.Background(), req, args...)
	if err != nil {
		return fmt.Errorf("%v.StreamingOutputCall(_) = _, %v", tc, err)
	}
	var rpcStatus error
	var respCnt int
	var index int
	for {
		reply, err := stream.Recv()
		if err != nil {
			rpcStatus = err
			break
		}
		t := reply.GetPayload().GetType()
		if t != testpb.PayloadType_COMPRESSABLE {
			return fmt.Errorf("Got the reply of type %d, want %d", t, testpb.PayloadType_COMPRESSABLE)
		}
		size := len(reply.GetPayload().GetBody())
		if size != respSizes[index] {
			return fmt.Errorf("Got reply body of length %d, want %d", size, respSizes[index])
		}
		index++
		respCnt++
	}
	if rpcStatus != io.EOF {
		return fmt.Errorf("Failed to finish the server streaming rpc: %v", rpcStatus)
	}
	if respCnt != len(respSizes) {
		return fmt.Errorf("Got %d reply, want %d", len(respSizes), respCnt)
	}
	return nil
}

// DoPingPong performs ping-pong style bi-directional streaming RPC.
func DoPingPong(tc testpb.TestServiceClient, args ...grpc.CallOption) error {
	stream, err := tc.FullDuplexCall(context.Background(), args...)
	if err != nil {
		return fmt.Errorf("%v.FullDuplexCall(_) = _, %v", tc, err)
	}
	var index int
	for index < len(reqSizes) {
		respParam := []*testpb.ResponseParameters{
			{
				Size: int32(respSizes[index]),
			},
		}
		pl, err := ClientNewPayload(testpb.PayloadType_COMPRESSABLE, reqSizes[index])
		if err != nil {
			return err
		}
		req := &testpb.StreamingOutputCallRequest{
			ResponseType:       testpb.PayloadType_COMPRESSABLE,
			ResponseParameters: respParam,
			Payload:            pl,
		}
		if err := stream.Send(req); err != nil {
			return fmt.Errorf("%v has error %v while sending %v", stream, err, req)
		}
		reply, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("%v.Recv() = %v", stream, err)
		}
		t := reply.GetPayload().GetType()
		if t != testpb.PayloadType_COMPRESSABLE {
			return fmt.Errorf("Got the reply of type %d, want %d", t, testpb.PayloadType_COMPRESSABLE)
		}
		size := len(reply.GetPayload().GetBody())
		if size != respSizes[index] {
			return fmt.Errorf("Got reply body of length %d, want %d", size, respSizes[index])
		}
		index++
	}
	if err := stream.CloseSend(); err != nil {
		return fmt.Errorf("%v.CloseSend() got %v, want %v", stream, err, nil)
	}
	if _, err := stream.Recv(); err != io.EOF {
		return fmt.Errorf("%v failed to complele the ping pong test: %v", stream, err)
	}
	return nil
}

// DoTimeoutOnSleepingServer performs an RPC on a sleep server which causes RPC timeout.
func DoTimeoutOnSleepingServer(tc testpb.TestServiceClient, args ...grpc.CallOption) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	stream, err := tc.FullDuplexCall(ctx, args...)
	if err != nil {
		if status.Code(err) == codes.DeadlineExceeded {
			return err
		}
		return fmt.Errorf("%v.FullDuplexCall(_) = _, %v", tc, err)
	}
	pl, err := ClientNewPayload(testpb.PayloadType_COMPRESSABLE, 27182)
	if err != nil {
		return err
	}
	req := &testpb.StreamingOutputCallRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		Payload:      pl,
	}
	if err := stream.Send(req); err != nil && err != io.EOF {
		return fmt.Errorf("%v.Send(_) = %v", stream, err)
	}
	if _, err := stream.Recv(); status.Code(err) != codes.DeadlineExceeded {
		return fmt.Errorf("%v.Recv() = _, %v, want error code %d", stream, err, codes.DeadlineExceeded)
	}
	return nil
}

var testMetadata = metadata.MD{
	"key1": []string{"value1"},
	"key2": []string{"value2"},
}

// DoCancelAfterBegin cancels the RPC after metadata has been sent but before payloads are sent.
func DoCancelAfterBegin(tc testpb.TestServiceClient, args ...grpc.CallOption) error {
	ctx, cancel := context.WithCancel(metadata.NewOutgoingContext(context.Background(), testMetadata))
	stream, err := tc.StreamingInputCall(ctx, args...)
	if err != nil {
		cancel()
		return fmt.Errorf("%v.StreamingInputCall(_) = _, %v", tc, err)
	}
	cancel()
	_, err = stream.CloseAndRecv()
	if status.Code(err) != codes.Canceled {
		return fmt.Errorf("%v.CloseAndRecv() got error code %d, want %d", stream, status.Code(err), codes.Canceled)
	}
	return nil
}

// DoCancelAfterFirstResponse cancels the RPC after receiving the first message from the server.
func DoCancelAfterFirstResponse(tc testpb.TestServiceClient, args ...grpc.CallOption) error {
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := tc.FullDuplexCall(ctx, args...)
	if err != nil {
		cancel()
		return fmt.Errorf("%v.FullDuplexCall(_) = _, %v", tc, err)
	}
	respParam := []*testpb.ResponseParameters{
		{
			Size: 31415,
		},
	}
	pl, err := ClientNewPayload(testpb.PayloadType_COMPRESSABLE, 27182)
	if err != nil {
		cancel()
		return err
	}
	req := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
		Payload:            pl,
	}
	if err := stream.Send(req); err != nil {
		cancel()
		return fmt.Errorf("%v has error %v while sending %v", stream, err, req)
	}
	if _, err := stream.Recv(); err != nil {
		cancel()
		return fmt.Errorf("%v.Recv() = %v", stream, err)
	}
	cancel()
	if _, err := stream.Recv(); status.Code(err) != codes.Canceled {
		return fmt.Errorf("%v compleled with error code %d, want %d", stream, status.Code(err), codes.Canceled)
	}
	return nil
}

var (
	initialMetadataValue  = "test_initial_metadata_value"
	trailingMetadataValue = "\x0a\x0b\x0a\x0b\x0a\x0b"
	customMetadata        = metadata.Pairs(
		initialMetadataKey, initialMetadataValue,
		trailingMetadataKey, trailingMetadataValue,
	)
)

func validateMetadata(header, trailer metadata.MD) error {
	if len(header[initialMetadataKey]) != 1 {
		return fmt.Errorf("Expected exactly one header from server. Received %d", len(header[initialMetadataKey]))
	}
	if header[initialMetadataKey][0] != initialMetadataValue {
		return fmt.Errorf("Got header %s; want %s", header[initialMetadataKey][0], initialMetadataValue)
	}
	if len(trailer[trailingMetadataKey]) != 1 {
		return fmt.Errorf("Expected exactly one trailer from server. Received %d", len(trailer[trailingMetadataKey]))
	}
	if trailer[trailingMetadataKey][0] != trailingMetadataValue {
		return fmt.Errorf("Got trailer %s; want %s", trailer[trailingMetadataKey][0], trailingMetadataValue)
	}
	return nil
}

// DoCustomMetadata checks that metadata is echoed back to the client.
func DoCustomMetadata(tc testpb.TestServiceClient, args ...grpc.CallOption) error {
	// Testing with UnaryCall.
	pl, err := ClientNewPayload(testpb.PayloadType_COMPRESSABLE, 1)
	if err != nil {
		return err
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: int32(1),
		Payload:      pl,
	}
	ctx := metadata.NewOutgoingContext(context.Background(), customMetadata)
	var header, trailer metadata.MD
	args = append(args, grpc.Header(&header), grpc.Trailer(&trailer))
	reply, err := tc.UnaryCall(
		ctx,
		req,
		args...,
	)
	if err != nil {
		return fmt.Errorf("/TestService/UnaryCall RPC failed: %v", err)
	}
	t := reply.GetPayload().GetType()
	s := len(reply.GetPayload().GetBody())
	if t != testpb.PayloadType_COMPRESSABLE || s != 1 {
		return fmt.Errorf("Got the reply with type %d len %d; want %d, %d", t, s, testpb.PayloadType_COMPRESSABLE, 1)
	}
	validateMetadata(header, trailer)

	// Testing with FullDuplex.
	stream, err := tc.FullDuplexCall(ctx, args...)
	if err != nil {
		return fmt.Errorf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	respParam := []*testpb.ResponseParameters{
		{
			Size: 1,
		},
	}
	streamReq := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
		Payload:            pl,
	}
	if err := stream.Send(streamReq); err != nil {
		return fmt.Errorf("%v has error %v while sending %v", stream, err, streamReq)
	}
	streamHeader, err := stream.Header()
	if err != nil {
		return fmt.Errorf("%v.Header() = %v", stream, err)
	}
	if _, err := stream.Recv(); err != nil {
		return fmt.Errorf("%v.Recv() = %v", stream, err)
	}
	if err := stream.CloseSend(); err != nil {
		return fmt.Errorf("%v.CloseSend() = %v, want <nil>", stream, err)
	}
	if _, err := stream.Recv(); err != io.EOF {
		return fmt.Errorf("%v failed to complete the custom metadata test: %v", stream, err)
	}
	streamTrailer := stream.Trailer()
	validateMetadata(streamHeader, streamTrailer)
	return nil
}

// DoStatusCodeAndMessage checks that the status code is propagated back to the client.
func DoStatusCodeAndMessage(tc testpb.TestServiceClient, args ...grpc.CallOption) error {
	var code int32 = 2
	msg := "test status message"
	expectedErr := status.Error(codes.Code(code), msg)
	respStatus := &testpb.EchoStatus{
		Code:    code,
		Message: msg,
	}
	// Test UnaryCall.
	req := &testpb.SimpleRequest{
		ResponseStatus: respStatus,
	}
	if _, err := tc.UnaryCall(context.Background(), req, args...); err == nil || err.Error() != expectedErr.Error() {
		return fmt.Errorf("%v.UnaryCall(_, %v) = _, %v, want _, %v", tc, req, err, expectedErr)
	}
	// Test FullDuplexCall.
	stream, err := tc.FullDuplexCall(context.Background(), args...)
	if err != nil {
		return fmt.Errorf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	streamReq := &testpb.StreamingOutputCallRequest{
		ResponseStatus: respStatus,
	}
	if err := stream.Send(streamReq); err != nil {
		return fmt.Errorf("%v has error %v while sending %v, want <nil>", stream, err, streamReq)
	}
	if err := stream.CloseSend(); err != nil {
		return fmt.Errorf("%v.CloseSend() = %v, want <nil>", stream, err)
	}
	if _, err = stream.Recv(); err.Error() != expectedErr.Error() {
		return fmt.Errorf("%v.Recv() returned error %v, want %v", stream, err, expectedErr)
	}
	return nil
}

// DoSpecialStatusMessage verifies Unicode and whitespace is correctly processed
// in status message.
func DoSpecialStatusMessage(tc testpb.TestServiceClient, args ...grpc.CallOption) error {
	const (
		code int32  = 2
		msg  string = "\t\ntest with whitespace\r\nand Unicode BMP ☺ and non-BMP 😈\t\n"
	)
	expectedErr := status.Error(codes.Code(code), msg)
	req := &testpb.SimpleRequest{
		ResponseStatus: &testpb.EchoStatus{
			Code:    code,
			Message: msg,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := tc.UnaryCall(ctx, req, args...); err == nil || err.Error() != expectedErr.Error() {
		return fmt.Errorf("%v.UnaryCall(_, %v) = _, %v, want _, %v", tc, req, err, expectedErr)
	}
	return nil
}

// DoUnimplementedService attempts to call a method from an unimplemented service.
func DoUnimplementedService(tc testpb.UnimplementedServiceClient) error {
	_, err := tc.UnimplementedCall(context.Background(), &testpb.Empty{})
	if status.Code(err) != codes.Unimplemented {
		return fmt.Errorf("%v.UnimplementedCall() = _, %v, want _, %v", tc, status.Code(err), codes.Unimplemented)
	}
	return nil
}

// DoUnimplementedMethod attempts to call an unimplemented method.
func DoUnimplementedMethod(cc *grpc.ClientConn) error {
	var req, reply proto.Message
	if err := cc.Invoke(context.Background(), "/grpc.testing.TestService/UnimplementedCall", req, reply); err == nil || status.Code(err) != codes.Unimplemented {
		return fmt.Errorf("ClientConn.Invoke(_, _, _, _, _) = %v, want error code %s", err, codes.Unimplemented)
	}
	return nil
}
