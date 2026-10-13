package grpc_interop

import (
	"errors"

	"google.golang.org/grpc"
	testpb "google.golang.org/grpc/interop/grpc_testing"
)

func ExecTestCase(conn *grpc.ClientConn, testCase string) error {
	tc := testpb.NewTestServiceClient(conn)

	switch testCase {
	case "cancel_after_begin":
		return DoCancelAfterBegin(tc)
	case "cancel_after_first_response":
		return DoCancelAfterFirstResponse(tc)
	case "client_streaming":
		return DoClientStreaming(tc)
	case "custom_metadata":
		return DoCustomMetadata(tc)
	case "empty_unary":
		return DoEmptyUnaryCall(tc)
	case "large_unary":
		return DoLargeUnaryCall(tc)
	case "ping_pong":
		return DoPingPong(tc)
	case "server_streaming":
		return DoServerStreaming(tc)
	case "special_status_message":
		return DoSpecialStatusMessage(tc)
	case "status_code_and_message":
		return DoStatusCodeAndMessage(tc)
	case "timeout_on_sleeping_server":
		return DoTimeoutOnSleepingServer(tc)
	case "unimplemented_method":
		return DoUnimplementedMethod(conn)
	case "unimplemented_service":
		return DoUnimplementedService(testpb.NewUnimplementedServiceClient(conn))
	default:
		return errors.New("invalid test name")
	}
}
