package helpers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

type membershipFactory struct {
	EtcdClientCreator
	client *clientv3.Client
	err    error
}

func (f membershipFactory) NewEtcdClient() (*clientv3.Client, func(), error) {
	return f.client, func() {}, f.err
}

type membershipCluster struct {
	clientv3.Cluster
	members []*etcdserverpb.Member
	err     error
}

func (c membershipCluster) MemberList(context.Context, ...clientv3.OpOption) (*clientv3.MemberListResponse, error) {
	return &clientv3.MemberListResponse{Members: c.members}, c.err
}

type membershipLog struct{ strings.Builder }

func (l *membershipLog) Logf(format string, args ...any) {
	fmt.Fprintf(&l.Builder, format+"\n", args...)
}

func TestMembershipDiagnosticsDoNotExposeClusterData(t *testing.T) {
	const sensitive = "internal-host.example"
	for _, tc := range []struct {
		name       string
		factoryErr bool
		memberErr  bool
		configErr  bool
		wantCount  int
		endpoints  map[string]string
		wantReady  bool
		wantErr    bool
	}{
		{name: "client error", factoryErr: true, wantCount: 1},
		{name: "member error", memberErr: true, wantCount: 1},
		{name: "member count mismatch", wantCount: 2},
		{name: "endpoint retrieval error", configErr: true, wantCount: 1, wantErr: true},
		{name: "endpoint count mismatch", wantCount: 1, endpoints: map[string]string{"1": sensitive, "2": "192.0.2.2"}},
		{name: "matching counts", wantCount: 1, endpoints: map[string]string{"1": sensitive}, wantReady: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := clientv3.NewCtxClient(t.Context())
			defer client.Close()
			cluster := membershipCluster{members: []*etcdserverpb.Member{{Name: sensitive}, {Name: sensitive + "-learner", IsLearner: true}}}
			if tc.memberErr {
				cluster.err = errors.New(sensitive)
			}
			client.Cluster = cluster
			factory := membershipFactory{client: client}
			if tc.factoryErr {
				factory.err = errors.New(sensitive)
			}
			kubeClient := kubefake.NewSimpleClientset(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: "openshift-etcd", Name: "etcd-endpoints"},
				Data:       tc.endpoints,
			})
			if tc.configErr {
				kubeClient.PrependReactor("get", "configmaps", func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New(sensitive)
				})
			}
			log := &membershipLog{}

			ready, err := checkVotingMembersCount(t.Context(), log, factory, kubeClient, tc.wantCount)

			if ready != tc.wantReady || (err != nil) != tc.wantErr {
				t.Errorf("membership check = %t, %v; want %t, error=%t", ready, err, tc.wantReady, tc.wantErr)
			}
			diagnostics := log.String()
			if err != nil {
				diagnostics += err.Error()
			}
			for _, value := range []string{sensitive, "192.0.2.2"} {
				if strings.Contains(diagnostics, value) {
					t.Errorf("membership diagnostics include cluster data: %s", diagnostics)
				}
			}
		})
	}
}
