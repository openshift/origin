package certs

import (
	"context"
	"reflect"
	"testing"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

func Test_mergeOneRawPKILists(t *testing.T) {
	type args struct {
		existingPKIList *certgraphapi.PKIList
		newPKIList      *certgraphapi.PKIList
	}
	tests := []struct {
		name  string
		args  args
		want  *certgraphapi.PKIList
		want1 []error
	}{
		{
			name: "add-fresh-certs",
			args: args{
				existingPKIList: newPKIList().toPKIListPtr(),
				newPKIList: newPKIList().
					withCert(
						newCertKeyPair().
							inCluster("openshift-config", "alpha").
							inCluster("openshift-kube-apiserver", "bravo").
							onDiskLocation("/other-serving.crt", "/other-serving.key").
							withPublicKeyModulus("carrot").
							toCertKeyPair(),
					).
					withCert(
						newCertKeyPair().
							inCluster("openshift-config", "charlie").
							inCluster("openshift-kube-controller-manager", "delta").
							inCluster("openshift-config-managed", "echo").
							onDiskLocation("/csr.crt", "/csr.key").
							withPublicKeyModulus("drumstick").
							toCertKeyPair(),
					).
					withCert(
						newCertKeyPair().
							inCluster("openshift-etcd", "foxtrot").
							onDiskLocation("/peer.crt", "/peer.key").
							withPublicKeyModulus("eggplant").
							toCertKeyPair(),
					).
					toPKIListPtr(),
			},
			want: newPKIList().
				withCert(
					newCertKeyPair().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						onDiskLocation("/other-serving.crt", "/other-serving.key").
						withPublicKeyModulus("carrot").
						toCertKeyPair(),
				).
				withCert(
					newCertKeyPair().
						inCluster("openshift-config", "charlie").
						inCluster("openshift-kube-controller-manager", "delta").
						inCluster("openshift-config-managed", "echo").
						onDiskLocation("/csr.crt", "/csr.key").
						withPublicKeyModulus("drumstick").
						toCertKeyPair(),
				).
				withCert(
					newCertKeyPair().
						inCluster("openshift-etcd", "foxtrot").
						onDiskLocation("/peer.crt", "/peer.key").
						withPublicKeyModulus("eggplant").
						toCertKeyPair(),
				).
				toPKIListPtr(),
			want1: []error{},
		},
		{
			name: "merge-and-map-certs",
			args: args{
				existingPKIList: newPKIList().
					withCert(
						newCertKeyPair().
							inCluster("openshift-config", "alpha").
							inCluster("openshift-kube-apiserver", "bravo").
							onDiskLocation("/serving.crt", "/serving.key").
							withPublicKeyModulus("apple").
							toCertKeyPair(),
					).
					withCert(
						newCertKeyPair().
							inCluster("openshift-config", "charlie").
							inCluster("openshift-kube-controller-manager", "delta").
							onDiskLocation("/csr.crt", "/csr.key").
							withPublicKeyModulus("banana").
							toCertKeyPair(),
					).
					toPKIListPtr(),
				newPKIList: newPKIList().
					withCert(
						newCertKeyPair().
							inCluster("openshift-config", "alpha").
							inCluster("openshift-kube-apiserver", "bravo").
							onDiskLocation("/other-serving.crt", "/other-serving.key").
							withPublicKeyModulus("carrot").
							toCertKeyPair(),
					).
					withCert(
						newCertKeyPair().
							inCluster("openshift-config", "charlie").
							inCluster("openshift-kube-controller-manager", "delta").
							inCluster("openshift-config-managed", "echo").
							onDiskLocation("/csr.crt", "/csr.key").
							withPublicKeyModulus("drumstick").
							toCertKeyPair(),
					).
					withCert(
						newCertKeyPair().
							inCluster("openshift-etcd", "foxtrot").
							onDiskLocation("/peer.crt", "/peer.key").
							withPublicKeyModulus("eggplant").
							toCertKeyPair(),
					).
					toPKIListPtr(),
			},
			want: newPKIList().
				withCert(
					newCertKeyPair().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						onDiskLocation("/serving.crt", "/serving.key").
						onDiskLocation("/other-serving.crt", "/other-serving.key").
						withPublicKeyModulus("apple").
						toCertKeyPair(),
				).
				withCert(
					newCertKeyPair().
						inCluster("openshift-config", "charlie").
						inCluster("openshift-kube-controller-manager", "delta").
						inCluster("openshift-config-managed", "echo").
						onDiskLocation("/csr.crt", "/csr.key").
						withPublicKeyModulus("banana").
						toCertKeyPair(),
				).
				withCert(
					newCertKeyPair().
						inCluster("openshift-etcd", "foxtrot").
						onDiskLocation("/peer.crt", "/peer.key").
						withPublicKeyModulus("eggplant").
						toCertKeyPair(),
				).
				toPKIListPtr(),
			want1: []error{},
		},
		{
			name: "merge-and-map-ca-bundles",
			args: args{
				existingPKIList: newPKIList().
					withCert(
						newCertKeyPair().
							inCluster("openshift-config", "alpha").
							inCluster("openshift-kube-apiserver", "bravo").
							onDiskLocation("/serving.crt", "/serving.key").
							withPublicKeyModulus("apple").
							toCertKeyPair(),
					).
					withCert(
						newCertKeyPair().
							inCluster("openshift-config", "charlie").
							inCluster("openshift-kube-controller-manager", "delta").
							onDiskLocation("/csr.crt", "/csr.key").
							withPublicKeyModulus("banana").
							toCertKeyPair(),
					).
					withCABundle(
						newCABundle().
							inCluster("openshift-config-managed", "golf").
							inCluster("openshift-etcd", "hotel").
							onDiskLocation("/ca.crt").
							withPublicKeyModulus("apple").
							toCABundle(),
					).
					toPKIListPtr(),
				newPKIList: newPKIList().
					withCert(
						newCertKeyPair().
							inCluster("openshift-config", "alpha").
							inCluster("openshift-kube-apiserver", "bravo").
							onDiskLocation("/other-serving.crt", "/other-serving.key").
							withPublicKeyModulus("carrot").
							toCertKeyPair(),
					).
					withCert(
						newCertKeyPair().
							inCluster("openshift-config", "charlie").
							inCluster("openshift-kube-controller-manager", "delta").
							inCluster("openshift-config-managed", "echo").
							onDiskLocation("/csr.crt", "/csr.key").
							withPublicKeyModulus("drumstick").
							toCertKeyPair(),
					).
					withCert(
						newCertKeyPair().
							inCluster("openshift-etcd", "foxtrot").
							onDiskLocation("/peer.crt", "/peer.key").
							withPublicKeyModulus("eggplant").
							toCertKeyPair(),
					).
					withCABundle(
						newCABundle().
							inCluster("openshift-config-managed", "golf").
							inCluster("openshift-etcd", "hotel").
							onDiskLocation("/ca.crt").
							toCABundle(),
					).
					withCABundle(
						newCABundle().
							inCluster("openshift-config-managed", "india").
							inCluster("openshift-etcd", "hotel").
							onDiskLocation("/ca-bundle.crt").
							withPublicKeyModulus("carrot").
							withPublicKeyModulus("eggplant").
							toCABundle(),
					).
					withCABundle(
						newCABundle().
							inCluster("openshift-config-managed", "juliet").
							withPublicKeyModulus("eggplant").
							toCABundle(),
					).
					withCABundle(
						newCABundle().
							inCluster("openshift-config-managed", "kilo").
							withPublicKeyModulus("drumstick").
							toCABundle(),
					).
					toPKIListPtr(),
			},
			want: newPKIList().
				withCert(
					newCertKeyPair().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						onDiskLocation("/serving.crt", "/serving.key").
						onDiskLocation("/other-serving.crt", "/other-serving.key").
						withPublicKeyModulus("apple"). // carrot gets mapped
						toCertKeyPair(),
				).
				withCert(
					newCertKeyPair().
						inCluster("openshift-config", "charlie").
						inCluster("openshift-kube-controller-manager", "delta").
						inCluster("openshift-config-managed", "echo").
						onDiskLocation("/csr.crt", "/csr.key").
						withPublicKeyModulus("banana"). // drumstick gets mapped
						toCertKeyPair(),
				).
				withCert(
					newCertKeyPair().
						inCluster("openshift-etcd", "foxtrot").
						onDiskLocation("/peer.crt", "/peer.key").
						withPublicKeyModulus("eggplant").
						toCertKeyPair(),
				).
				withCABundle(
					newCABundle().
						inCluster("openshift-config-managed", "golf").
						inCluster("openshift-etcd", "hotel").
						inCluster("openshift-config-managed", "india").
						onDiskLocation("/ca.crt").
						onDiskLocation("/ca-bundle.crt").
						withPublicKeyModulus("apple"). // carrot gets mapped
						withPublicKeyModulus("eggplant").
						toCABundle(),
				).
				withCABundle(
					newCABundle().
						inCluster("openshift-config-managed", "juliet").
						withPublicKeyModulus("eggplant").
						toCABundle(),
				).
				withCABundle(
					newCABundle().
						inCluster("openshift-config-managed", "kilo").
						withPublicKeyModulus("banana"). // ca bundle doesn't exist in the original, but the mapping still happens
						toCABundle(),
				).
				toPKIListPtr(),
			want1: []error{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := mergeOneRawPKILists(context.TODO(), tt.args.existingPKIList, tt.args.newPKIList)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeOneRawPKILists() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("mergeOneRawPKILists() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestCertIdentifierMappings_Map(t *testing.T) {
	type fields struct {
		Mappings []CertIdentifierMapping
	}
	type args struct {
		in certgraphapi.CertIdentifier
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *certgraphapi.CertIdentifier
		wantErr bool
	}{
		{
			name: "simple",
			fields: fields{
				Mappings: []CertIdentifierMapping{
					{
						FromValue: certgraphapi.CertIdentifier{PubkeyModulus: "carrot"},
						ToValue:   certgraphapi.CertIdentifier{PubkeyModulus: "apple"},
					},
					{
						FromValue: certgraphapi.CertIdentifier{PubkeyModulus: "drumstick"},
						ToValue:   certgraphapi.CertIdentifier{PubkeyModulus: "banana"},
					},
				},
			},
			args: args{
				in: certgraphapi.CertIdentifier{PubkeyModulus: "drumstick"},
			},
			want:    &certgraphapi.CertIdentifier{PubkeyModulus: "banana"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CertIdentifierMappings{
				Mappings: tt.fields.Mappings,
			}
			got, err := c.Map(tt.args.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("Map() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Map() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_categorizeCertKeys(t *testing.T) {
	type args struct {
		existingCertKeyPairs []certgraphapi.CertKeyPair
		newCertKeyPairs      []certgraphapi.CertKeyPair
	}
	tests := []struct {
		name  string
		args  args
		want  []certgraphapi.CertKeyPair
		want1 []certgraphapi.CertKeyPair
		want2 *CertIdentifierMappings
		want3 []error
	}{
		{
			name: "smattering",
			args: args{
				existingCertKeyPairs: []certgraphapi.CertKeyPair{
					newCertKeyPair().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						onDiskLocation("/serving.crt", "/serving.key").
						withPublicKeyModulus("apple").
						toCertKeyPair(),
					newCertKeyPair().
						inCluster("openshift-config", "charlie").
						inCluster("openshift-kube-controller-manager", "delta").
						onDiskLocation("/csr.crt", "/csr.key").
						withPublicKeyModulus("banana").
						toCertKeyPair(),
				},
				newCertKeyPairs: []certgraphapi.CertKeyPair{
					newCertKeyPair().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						onDiskLocation("/other-serving.crt", "/other-serving.key").
						withPublicKeyModulus("carrot").
						toCertKeyPair(),
					newCertKeyPair().
						inCluster("openshift-config", "charlie").
						inCluster("openshift-kube-controller-manager", "delta").
						inCluster("openshift-config-managed", "echo").
						onDiskLocation("/csr.crt", "/csr.key").
						withPublicKeyModulus("drumstick").
						toCertKeyPair(),
					newCertKeyPair().
						inCluster("openshift-etcd", "foxtrot").
						onDiskLocation("/peer.crt", "/peer.key").
						withPublicKeyModulus("eggplant").
						toCertKeyPair(),
				},
			},
			want: []certgraphapi.CertKeyPair{
				newCertKeyPair().
					inCluster("openshift-config", "alpha").
					inCluster("openshift-kube-apiserver", "bravo").
					onDiskLocation("/other-serving.crt", "/other-serving.key").
					withPublicKeyModulus("carrot").
					toCertKeyPair(),
				newCertKeyPair().
					inCluster("openshift-config", "charlie").
					inCluster("openshift-kube-controller-manager", "delta").
					inCluster("openshift-config-managed", "echo").
					onDiskLocation("/csr.crt", "/csr.key").
					withPublicKeyModulus("drumstick").
					toCertKeyPair(),
			},
			want1: []certgraphapi.CertKeyPair{
				newCertKeyPair().
					inCluster("openshift-etcd", "foxtrot").
					onDiskLocation("/peer.crt", "/peer.key").
					withPublicKeyModulus("eggplant").
					toCertKeyPair(),
			},
			want2: &CertIdentifierMappings{
				Mappings: []CertIdentifierMapping{
					{
						FromValue: certgraphapi.CertIdentifier{PubkeyModulus: "carrot"},
						ToValue:   certgraphapi.CertIdentifier{PubkeyModulus: "apple"},
					},
					{
						FromValue: certgraphapi.CertIdentifier{PubkeyModulus: "drumstick"},
						ToValue:   certgraphapi.CertIdentifier{PubkeyModulus: "banana"},
					},
				},
			},
			want3: []error{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, got3 := categorizeCertKeys(tt.args.existingCertKeyPairs, tt.args.newCertKeyPairs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("categorizeCertKeys() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("categorizeCertKeys() got1 = %v, want %v", got1, tt.want1)
			}
			if !reflect.DeepEqual(got2, tt.want2) {
				t.Errorf("categorizeCertKeys() got2 = %v, want %v", got2, tt.want2)
			}
			if !reflect.DeepEqual(got3, tt.want3) {
				t.Errorf("categorizeCertKeys() got3 = %v, want %v", got3, tt.want3)
			}
		})
	}
}

func Test_categorizeCABundles(t *testing.T) {
	type args struct {
		existingCABundles []certgraphapi.CertificateAuthorityBundle
		newCABundles      []certgraphapi.CertificateAuthorityBundle
	}
	tests := []struct {
		name  string
		args  args
		want  []certgraphapi.CertificateAuthorityBundle
		want1 []certgraphapi.CertificateAuthorityBundle
		want2 []error
	}{
		{
			name: "smattering",
			args: args{
				existingCABundles: []certgraphapi.CertificateAuthorityBundle{
					newCABundle().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						onDiskLocation("/ca-bundle.crt").
						withPublicKeyModulus("apple").
						toCABundle(),
					newCABundle().
						inCluster("openshift-config", "charlie").
						inCluster("openshift-kube-controller-manager", "delta").
						onDiskLocation("/csr.crt").
						withPublicKeyModulus("banana").
						toCABundle(),
				},
				newCABundles: []certgraphapi.CertificateAuthorityBundle{
					newCABundle().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						onDiskLocation("/ca.crt").
						withPublicKeyModulus("carrot").
						toCABundle(),
					newCABundle().
						inCluster("openshift-config", "charlie").
						inCluster("openshift-kube-controller-manager", "delta").
						inCluster("openshift-config-managed", "echo").
						onDiskLocation("/csr.crt").
						withPublicKeyModulus("drumstick").
						toCABundle(),
					newCABundle().
						inCluster("openshift-etcd", "foxtrot").
						onDiskLocation("/peer.crt").
						withPublicKeyModulus("eggplant").
						toCABundle(),
				},
			},
			want: []certgraphapi.CertificateAuthorityBundle{
				newCABundle().
					inCluster("openshift-config", "alpha").
					inCluster("openshift-kube-apiserver", "bravo").
					onDiskLocation("/ca.crt").
					withPublicKeyModulus("carrot").
					toCABundle(),
				newCABundle().
					inCluster("openshift-config", "charlie").
					inCluster("openshift-kube-controller-manager", "delta").
					inCluster("openshift-config-managed", "echo").
					onDiskLocation("/csr.crt").
					withPublicKeyModulus("drumstick").
					toCABundle(),
			},
			want1: []certgraphapi.CertificateAuthorityBundle{
				newCABundle().
					inCluster("openshift-etcd", "foxtrot").
					onDiskLocation("/peer.crt").
					withPublicKeyModulus("eggplant").
					toCABundle(),
			},
			want2: []error{},
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2 := categorizeCABundles(tt.args.existingCABundles, tt.args.newCABundles)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("categorizeCABundles() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("categorizeCABundles() got1 = %v, want %v", got1, tt.want1)
			}
			if !reflect.DeepEqual(got2, tt.want2) {
				t.Errorf("categorizeCABundles() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}

func TestLocateMatchingCertKeyPairs(t *testing.T) {
	type args struct {
		items  []certgraphapi.CertKeyPair
		target certgraphapi.CertKeyPair
	}
	tests := []struct {
		name string
		args args
		want []*certgraphapi.CertKeyPair
	}{
		{
			name: "single-match-secret",
			args: args{
				items: []certgraphapi.CertKeyPair{
					newCertKeyPair().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						withPublicKeyModulus("apple").
						toCertKeyPair(),
				},
				target: newCertKeyPair().
					inCluster("openshift-config", "alpha").
					withPublicKeyModulus("banana").
					toCertKeyPair(),
			},
			want: []*certgraphapi.CertKeyPair{
				newCertKeyPair().
					inCluster("openshift-config", "alpha").
					inCluster("openshift-kube-apiserver", "bravo").
					withPublicKeyModulus("apple").
					toCertKeyPairPtr(),
			},
		},
		{
			name: "double-match-secret",
			args: args{
				items: []certgraphapi.CertKeyPair{
					newCertKeyPair().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						withPublicKeyModulus("apple").
						toCertKeyPair(),
					newCertKeyPair().
						inCluster("openshift-config", "charlie").
						inCluster("openshift-config", "delta").
						withPublicKeyModulus("carrot").
						toCertKeyPair(),
				},
				target: newCertKeyPair().
					inCluster("openshift-config", "alpha").
					inCluster("openshift-config", "charlie").
					withPublicKeyModulus("banana").
					toCertKeyPair(),
			},
			want: []*certgraphapi.CertKeyPair{
				newCertKeyPair().
					inCluster("openshift-config", "alpha").
					inCluster("openshift-kube-apiserver", "bravo").
					withPublicKeyModulus("apple").
					toCertKeyPairPtr(),
				newCertKeyPair().
					inCluster("openshift-config", "charlie").
					inCluster("openshift-config", "delta").
					withPublicKeyModulus("carrot").
					toCertKeyPairPtr(),
			},
		},
		{
			name: "no-match",
			args: args{
				items: []certgraphapi.CertKeyPair{
					newCertKeyPair().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						withPublicKeyModulus("apple").
						toCertKeyPair(),
				},
				target: newCertKeyPair().
					inCluster("openshift-config", "charlie").
					withPublicKeyModulus("banana").
					toCertKeyPair(),
			},
			want: []*certgraphapi.CertKeyPair{},
		},
		{
			name: "single-match-on-cert",
			args: args{
				items: []certgraphapi.CertKeyPair{
					newCertKeyPair().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						onDiskLocation("/cert.crt", "/key.key").
						withPublicKeyModulus("apple").
						toCertKeyPair(),
				},
				target: newCertKeyPair().
					onDiskLocation("/cert.crt", "").
					withPublicKeyModulus("banana").
					toCertKeyPair(),
			},
			want: []*certgraphapi.CertKeyPair{
				newCertKeyPair().
					inCluster("openshift-config", "alpha").
					inCluster("openshift-kube-apiserver", "bravo").
					onDiskLocation("/cert.crt", "/key.key").
					withPublicKeyModulus("apple").
					toCertKeyPairPtr(),
			},
		},
		{
			name: "single-match-on-cert-2",
			args: args{
				items: []certgraphapi.CertKeyPair{
					newCertKeyPair().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						onDiskLocation("/cert.crt", "/key.key").
						withPublicKeyModulus("apple").
						toCertKeyPair(),
				},
				target: newCertKeyPair().
					onDiskLocation("/cert.crt", "not-key").
					withPublicKeyModulus("banana").
					toCertKeyPair(),
			},
			want: []*certgraphapi.CertKeyPair{
				newCertKeyPair().
					inCluster("openshift-config", "alpha").
					inCluster("openshift-kube-apiserver", "bravo").
					onDiskLocation("/cert.crt", "/key.key").
					withPublicKeyModulus("apple").
					toCertKeyPairPtr(),
			},
		},
		{
			name: "single-match-on-key",
			args: args{
				items: []certgraphapi.CertKeyPair{
					newCertKeyPair().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						onDiskLocation("/cert.crt", "/key.key").
						withPublicKeyModulus("apple").
						toCertKeyPair(),
				},
				target: newCertKeyPair().
					onDiskLocation("", "/key.key").
					withPublicKeyModulus("banana").
					toCertKeyPair(),
			},
			want: []*certgraphapi.CertKeyPair{
				newCertKeyPair().
					inCluster("openshift-config", "alpha").
					inCluster("openshift-kube-apiserver", "bravo").
					onDiskLocation("/cert.crt", "/key.key").
					withPublicKeyModulus("apple").
					toCertKeyPairPtr(),
			},
		},
		{
			name: "dedupe-same-double-match",
			args: args{
				items: []certgraphapi.CertKeyPair{
					newCertKeyPair().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						onDiskLocation("/cert.crt", "/key.key").
						withPublicKeyModulus("apple").
						toCertKeyPair(),
				},
				target: newCertKeyPair().
					inCluster("openshift-kube-apiserver", "bravo").
					onDiskLocation("/cert.crt", "/key.key").
					withPublicKeyModulus("banana").
					toCertKeyPair(),
			},
			want: []*certgraphapi.CertKeyPair{
				newCertKeyPair().
					inCluster("openshift-config", "alpha").
					inCluster("openshift-kube-apiserver", "bravo").
					onDiskLocation("/cert.crt", "/key.key").
					withPublicKeyModulus("apple").
					toCertKeyPairPtr(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LocateMatchingCertKeyPairs(tt.args.items, tt.args.target); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LocateMatchingCertKeyPairs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocateMatchingCertificateAuthorityBundles(t *testing.T) {
	type args struct {
		items  []certgraphapi.CertificateAuthorityBundle
		target certgraphapi.CertificateAuthorityBundle
	}
	tests := []struct {
		name string
		args args
		want []*certgraphapi.CertificateAuthorityBundle
	}{
		{
			name: "single-match-configmap",
			args: args{
				items: []certgraphapi.CertificateAuthorityBundle{
					newCABundle().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						withPublicKeyModulus("apple").
						toCABundle(),
				},
				target: newCABundle().
					inCluster("openshift-config", "alpha").
					withPublicKeyModulus("banana").
					toCABundle(),
			},
			want: []*certgraphapi.CertificateAuthorityBundle{
				newCABundle().
					inCluster("openshift-config", "alpha").
					inCluster("openshift-kube-apiserver", "bravo").
					withPublicKeyModulus("apple").
					toCABundlePtr(),
			},
		},
		{
			name: "double-match-configmap",
			args: args{
				items: []certgraphapi.CertificateAuthorityBundle{
					newCABundle().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						withPublicKeyModulus("apple").
						toCABundle(),
					newCABundle().
						inCluster("openshift-config", "charlie").
						inCluster("openshift-config", "delta").
						withPublicKeyModulus("carrot").
						toCABundle(),
				},
				target: newCABundle().
					inCluster("openshift-config", "alpha").
					inCluster("openshift-config", "charlie").
					withPublicKeyModulus("banana").
					toCABundle(),
			},
			want: []*certgraphapi.CertificateAuthorityBundle{
				newCABundle().
					inCluster("openshift-config", "alpha").
					inCluster("openshift-kube-apiserver", "bravo").
					withPublicKeyModulus("apple").
					toCABundlePtr(),
				newCABundle().
					inCluster("openshift-config", "charlie").
					inCluster("openshift-config", "delta").
					withPublicKeyModulus("carrot").
					toCABundlePtr(),
			},
		},
		{
			name: "no-match",
			args: args{
				items: []certgraphapi.CertificateAuthorityBundle{
					newCABundle().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						withPublicKeyModulus("apple").
						toCABundle(),
				},
				target: newCABundle().
					inCluster("openshift-config", "charlie").
					withPublicKeyModulus("banana").
					toCABundle(),
			},
			want: []*certgraphapi.CertificateAuthorityBundle{},
		},
		{
			name: "single-match-on-cert",
			args: args{
				items: []certgraphapi.CertificateAuthorityBundle{
					newCABundle().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						onDiskLocation("/ca-bundle.crt").
						withPublicKeyModulus("apple").
						toCABundle(),
				},
				target: newCABundle().
					onDiskLocation("/ca-bundle.crt").
					withPublicKeyModulus("banana").
					toCABundle(),
			},
			want: []*certgraphapi.CertificateAuthorityBundle{
				newCABundle().
					inCluster("openshift-config", "alpha").
					inCluster("openshift-kube-apiserver", "bravo").
					onDiskLocation("/ca-bundle.crt").
					withPublicKeyModulus("apple").
					toCABundlePtr(),
			},
		},
		{
			name: "dedupe-same-double-match",
			args: args{
				items: []certgraphapi.CertificateAuthorityBundle{
					newCABundle().
						inCluster("openshift-config", "alpha").
						inCluster("openshift-kube-apiserver", "bravo").
						onDiskLocation("/ca-bundle.crt").
						withPublicKeyModulus("apple").
						toCABundle(),
				},
				target: newCABundle().
					inCluster("openshift-kube-apiserver", "bravo").
					onDiskLocation("/ca-bundle.crt").
					withPublicKeyModulus("banana").
					toCABundle(),
			},
			want: []*certgraphapi.CertificateAuthorityBundle{
				newCABundle().
					inCluster("openshift-config", "alpha").
					inCluster("openshift-kube-apiserver", "bravo").
					onDiskLocation("/ca-bundle.crt").
					withPublicKeyModulus("apple").
					toCABundlePtr(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LocateMatchingCertificateAuthorityBundles(tt.args.items, tt.args.target); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LocateMatchingCertificateAuthorityBundles() = %v, want %v", got, tt.want)
			}
		})
	}
}

type pkiListBuilder struct {
	certs     []certgraphapi.CertKeyPair
	caBundles []certgraphapi.CertificateAuthorityBundle
}

func newPKIList() *pkiListBuilder {
	return &pkiListBuilder{}
}

func (b *pkiListBuilder) withCert(cert certgraphapi.CertKeyPair) *pkiListBuilder {
	b.certs = append(b.certs, *cert.DeepCopy())
	return b
}

func (b *pkiListBuilder) withCABundle(caBundle certgraphapi.CertificateAuthorityBundle) *pkiListBuilder {
	b.caBundles = append(b.caBundles, *caBundle.DeepCopy())
	return b
}

func (b *pkiListBuilder) toPKIListPtr() *certgraphapi.PKIList {
	ret := &certgraphapi.PKIList{
		CertificateAuthorityBundles: certgraphapi.CertificateAuthorityBundleList{
			Items: b.caBundles,
		},
		CertKeyPairs: certgraphapi.CertKeyPairList{
			Items: b.certs,
		},
	}
	return ret
}

type certKeyPairBuilder struct {
	inClusterLocation []certgraphapi.InClusterSecretLocation
	onDiskLocations   []certgraphapi.OnDiskCertKeyPairLocation
	publicKeyModulus  string
}

func newCertKeyPair() *certKeyPairBuilder {
	return &certKeyPairBuilder{}
}

func (b *certKeyPairBuilder) inCluster(namespace, name string) *certKeyPairBuilder {
	b.inClusterLocation = append(b.inClusterLocation, certgraphapi.InClusterSecretLocation{
		Namespace: namespace,
		Name:      name,
	})
	return b
}

func (b *certKeyPairBuilder) onDiskLocation(certPath, keyPath string) *certKeyPairBuilder {
	b.onDiskLocations = append(b.onDiskLocations, certgraphapi.OnDiskCertKeyPairLocation{
		Cert: certgraphapi.OnDiskLocation{
			Path: certPath,
		},
		Key: certgraphapi.OnDiskLocation{
			Path: keyPath,
		},
	})
	return b
}

func (b *certKeyPairBuilder) withPublicKeyModulus(val string) *certKeyPairBuilder {
	b.publicKeyModulus = val
	return b
}

func (b *certKeyPairBuilder) toCertKeyPair() certgraphapi.CertKeyPair {
	ret := certgraphapi.CertKeyPair{
		Spec: certgraphapi.CertKeyPairSpec{
			SecretLocations: b.inClusterLocation,
			OnDiskLocations: b.onDiskLocations,
			CertMetadata: certgraphapi.CertKeyMetadata{
				CertIdentifier: certgraphapi.CertIdentifier{
					PubkeyModulus: b.publicKeyModulus,
				},
			},
		},
	}
	return ret
}

func (b *certKeyPairBuilder) toCertKeyPairPtr() *certgraphapi.CertKeyPair {
	ret := b.toCertKeyPair()
	return &ret
}

type caBundleBuilder struct {
	inClusterLocation []certgraphapi.InClusterConfigMapLocation
	onDiskLocations   []certgraphapi.OnDiskLocation
	publicKeyModuli   []string
}

func newCABundle() *caBundleBuilder {
	return &caBundleBuilder{}
}

func (b *caBundleBuilder) inCluster(namespace, name string) *caBundleBuilder {
	b.inClusterLocation = append(b.inClusterLocation, certgraphapi.InClusterConfigMapLocation{
		Namespace: namespace,
		Name:      name,
	})
	return b
}

func (b *caBundleBuilder) onDiskLocation(path string) *caBundleBuilder {
	b.onDiskLocations = append(b.onDiskLocations, certgraphapi.OnDiskLocation{
		Path: path,
	})
	return b
}

func (b *caBundleBuilder) withPublicKeyModulus(val string) *caBundleBuilder {
	b.publicKeyModuli = append(b.publicKeyModuli, val)
	return b
}

func (b *caBundleBuilder) toCABundle() certgraphapi.CertificateAuthorityBundle {
	ret := certgraphapi.CertificateAuthorityBundle{
		Spec: certgraphapi.CertificateAuthorityBundleSpec{
			ConfigMapLocations: b.inClusterLocation,
			OnDiskLocations:    b.onDiskLocations,
		},
	}
	for _, curr := range b.publicKeyModuli {
		ret.Spec.CertificateMetadata = append(ret.Spec.CertificateMetadata, certgraphapi.CertKeyMetadata{
			CertIdentifier: certgraphapi.CertIdentifier{
				PubkeyModulus: curr,
			},
		})
	}
	return ret
}

func (b *caBundleBuilder) toCABundlePtr() *certgraphapi.CertificateAuthorityBundle {
	ret := b.toCABundle()
	return &ret
}
