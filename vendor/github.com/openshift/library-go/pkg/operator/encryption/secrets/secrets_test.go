package secrets

import (
	"encoding/base64"
	"reflect"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	v1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/utils/diff"

	"github.com/openshift/library-go/pkg/operator/encryption/state"
)

func TestRoundtrip(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))

	tests := []struct {
		name      string
		component string
		ks        state.KeyState
	}{
		{
			name:      "full aescbc",
			component: "kms",
			ks: state.KeyState{
				Key: v1.Key{
					Name:   "54",
					Secret: base64.StdEncoding.EncodeToString([]byte("abcdef")),
				},
				Backed: true, // this will be set by ToKeyState()
				Mode:   "aescbc",
				Migrated: state.MigrationState{
					Timestamp: now,
					Resources: []schema.GroupResource{
						{Resource: "secrets"},
						{Resource: "configmaps"},
						{Group: "networking.openshift.io", Resource: "routes"},
					},
				},
				InternalReason: "internal",
				ExternalReason: "external",
			},
		},
		{
			name:      "sparse aescbc",
			component: "kms",
			ks: state.KeyState{
				Key: v1.Key{
					Name:   "54",
					Secret: base64.StdEncoding.EncodeToString([]byte("abcdef")),
				},
				Backed: true, // this will be set by ToKeyState()
				Mode:   "aescbc",
			},
		},
		{
			name:      "identity",
			component: "kms",
			ks: state.KeyState{
				Key: v1.Key{
					Name:   "54",
					Secret: "",
				},
				Backed: true, // this will be set by ToKeyState()
				Mode:   "identity",
				Migrated: state.MigrationState{
					Timestamp: now,
					Resources: []schema.GroupResource{
						{Resource: "secrets"},
						{Resource: "configmaps"},
						{Group: "networking.openshift.io", Resource: "routes"},
					},
				},
				InternalReason: "internal",
				ExternalReason: "external",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := FromKeyState(tt.component, tt.ks)
			if err != nil {
				t.Fatalf("unexpected FromKeyState() error: %v", err)
			}
			got, err := ToKeyState(s)
			if err != nil {
				t.Fatalf("unexpected ToKeyState() error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.ks) {
				t.Errorf("roundtrip error:\n%s", diff.ObjectDiff(tt.ks, got))
			}
		})
	}
}
