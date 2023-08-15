package legacykubeapiservermonitortests

import (
	"reflect"
	"testing"
)

func Test_staticPodFailureFromMessage(t *testing.T) {
	type args struct {
		message string
	}
	tests := []struct {
		name    string
		args    args
		want    *staticPodFailure
		wantErr bool
	}{
		{
			name: "parser",
			args: args{message: `static pod lifecycle failure - static pod: "etcd" in namespace: "openshift-etcd" for revision: 6 on node: "ovirt10-gh8t5-master-2" didn't show up, waited: 2m30s`},
			want: &staticPodFailure{
				node:           "ovirt10-gh8t5-master-2",
				revision:       6,
				failureMessage: `static pod lifecycle failure - static pod: "etcd" in namespace: "openshift-etcd" for revision: 6 on node: "ovirt10-gh8t5-master-2" didn't show up, waited: 2m30s`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := staticPodFailureFromMessage(tt.args.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("staticPodFailureFromMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("staticPodFailureFromMessage() got = %v, want %v", got, tt.want)
			}
		})
	}
}
