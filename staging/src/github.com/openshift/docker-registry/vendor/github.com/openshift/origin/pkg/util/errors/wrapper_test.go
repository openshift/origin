package errors

import (
	"context"
	stderrors "errors"
	"math/rand"
	"reflect"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	"github.com/google/gofuzz"
)

func TestSyncStatusError(t *testing.T) {
	for i := 0; i < 100; i++ {
		for _, test := range []struct {
			name    string
			errFunc func(gr1, gr2 *schema.GroupResource, gk1, gk2 *schema.GroupKind) (inputErr *apierrors.StatusError, expectedErr *apierrors.StatusError)
		}{
			{
				name: "NewNotFound",
				errFunc: func(gr1, gr2 *schema.GroupResource, _, _ *schema.GroupKind) (*apierrors.StatusError, *apierrors.StatusError) {
					return apierrors.NewNotFound(*gr1, "someval"), apierrors.NewNotFound(*gr2, "someval")
				},
			},
			{
				name: "NewAlreadyExists",
				errFunc: func(gr1, gr2 *schema.GroupResource, _, _ *schema.GroupKind) (*apierrors.StatusError, *apierrors.StatusError) {
					return apierrors.NewAlreadyExists(*gr1, "someval"), apierrors.NewAlreadyExists(*gr2, "someval")
				},
			},
			{
				name: "NewForbidden",
				errFunc: func(gr1, gr2 *schema.GroupResource, _, _ *schema.GroupKind) (*apierrors.StatusError, *apierrors.StatusError) {
					return apierrors.NewForbidden(*gr1, "someval", stderrors.New("someerr")), apierrors.NewForbidden(*gr2, "someval", stderrors.New("someerr"))
				},
			},
			{
				name: "NewConflict",
				errFunc: func(gr1, gr2 *schema.GroupResource, _, _ *schema.GroupKind) (*apierrors.StatusError, *apierrors.StatusError) {
					return apierrors.NewConflict(*gr1, "someval", stderrors.New("someerr")), apierrors.NewConflict(*gr2, "someval", stderrors.New("someerr"))
				},
			},
			{
				name: "NewInvalid",
				errFunc: func(_, _ *schema.GroupResource, gk1, gk2 *schema.GroupKind) (*apierrors.StatusError, *apierrors.StatusError) {
					return apierrors.NewInvalid(*gk1, "someval", field.ErrorList{field.NotFound(field.NewPath("foo"), "bar")}),
						apierrors.NewInvalid(*gk2, "someval", field.ErrorList{field.NotFound(field.NewPath("foo"), "bar")})
				},
			},
			{
				name: "NewMethodNotSupported",
				errFunc: func(gr1, gr2 *schema.GroupResource, _, _ *schema.GroupKind) (*apierrors.StatusError, *apierrors.StatusError) {
					return apierrors.NewMethodNotSupported(*gr1, "someval"), apierrors.NewMethodNotSupported(*gr2, "someval")
				},
			},
			{
				name: "NewServerTimeout",
				errFunc: func(gr1, gr2 *schema.GroupResource, _, _ *schema.GroupKind) (*apierrors.StatusError, *apierrors.StatusError) {
					return apierrors.NewServerTimeout(*gr1, "someval", 1), apierrors.NewServerTimeout(*gr2, "someval", 1)
				},
			},
			{
				name: "NewServerTimeoutForKind",
				errFunc: func(_, _ *schema.GroupResource, gk1, gk2 *schema.GroupKind) (*apierrors.StatusError, *apierrors.StatusError) {
					return apierrors.NewServerTimeoutForKind(*gk1, "someval", 2), apierrors.NewServerTimeoutForKind(*gk2, "someval", 2)
				},
			},
			{
				name: "NewGenericServerResponse",
				errFunc: func(gr1, gr2 *schema.GroupResource, _, _ *schema.GroupKind) (*apierrors.StatusError, *apierrors.StatusError) {
					return apierrors.NewGenericServerResponse(401, "someval", *gr1, "someval", "someval", 3, true),
						apierrors.NewGenericServerResponse(401, "someval", *gr2, "someval", "someval", 3, true)
				},
			},
		} {
			info := &apirequest.RequestInfo{}
			gr1 := &schema.GroupResource{}
			gk1 := &schema.GroupKind{}

			fuzzall(
				info,
				gr1,
				gk1,
			)

			gr2 := &schema.GroupResource{Group: info.APIGroup, Resource: info.Resource}
			gk2 := &schema.GroupKind{Group: info.APIGroup, Kind: info.Resource}

			ctx := apirequest.WithRequestInfo(context.TODO(), info)
			inputErr, expectedErr := test.errFunc(gr1, gr2, gk1, gk2)
			actualErr := SyncStatusError(ctx, inputErr)
			if !reflect.DeepEqual(actualErr, expectedErr) {
				t.Errorf("Test %s at idx %d is not equal: %s", test.name, i, diff.ObjectGoPrintDiff(actualErr, expectedErr))
			}
		}
	}
}

var fuzzer = fuzz.New().NilChance(0).Funcs(
	func(s *string, c fuzz.Continue) {
		*s = nonEmptyQuoteSafeString(c.Rand)
	},
)

func fuzzall(objs ...interface{}) {
	for _, obj := range objs {
		fuzzer.Fuzz(obj)
	}
}

// charset contains all ASCII characters expect " and \
const charset = " !#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[]^_`abcdefghijklmnopqrstuvwxyz{|}~"

func nonEmptyQuoteSafeString(r *rand.Rand) string {
	n := r.Intn(20) + 5
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[r.Intn(len(charset))]
	}
	return string(b)
}
