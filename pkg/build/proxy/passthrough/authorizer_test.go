package passthrough

import "testing"

func Test_parseKubeCgroupParent(t *testing.T) {
	type args struct {
		parent string
	}
	tests := []struct {
		name         string
		args         args
		podUID       string
		containerID  string
		parentCgroup string
		wantErr      bool
	}{
		{
			name:         "parse valid Kubernetes slice",
			args:         args{parent: "/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-podcea81ac2_7d63_11e7_96cd_080027893417.slice/docker-0d9ba9436cdf7136eb3217baac1e6a41c1e0d15a10b5f51a3f8b3a53ea5e5e03.scope"},
			podUID:       "cea81ac2-7d63-11e7-96cd-080027893417",
			containerID:  "0d9ba9436cdf7136eb3217baac1e6a41c1e0d15a10b5f51a3f8b3a53ea5e5e03",
			parentCgroup: "kubepods-burstable-podcea81ac2_7d63_11e7_96cd_080027893417.slice",
			wantErr:      false,
		},
		{
			name:    "invalid QoS value",
			args:    args{parent: "/kubepods.slice/kubepods-.slice/kubepods--podcea81ac2_7d63_11e7_96cd_080027893417.slice/docker-0d9ba9436cdf7136eb3217baac1e6a41c1e0d15a10b5f51a3f8b3a53ea5e5e03.scope"},
			wantErr: true,
		},
		{
			name:    "missing pod UID",
			args:    args{parent: "/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod.slice/docker-0d9ba9436cdf7136eb3217baac1e6a41c1e0d15a10b5f51a3f8b3a53ea5e5e03.scope"},
			wantErr: true,
		},
		{
			name:    "missing container ID",
			args:    args{parent: "/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-podcea81ac2_7d63_11e7_96cd_080027893417.slice/docker-.scope"},
			wantErr: true,
		},
		{
			name:    "slice QoS tiers mismatch",
			args:    args{parent: "/kubepods.slice/kubepods-burstable.slice/kubepods-besteffort-podcea81ac2_7d63_11e7_96cd_080027893417.slice/docker-0d9ba9436cdf7136eb3217baac1e6a41c1e0d15a10b5f51a3f8b3a53ea5e5e03.scope"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podUID, containerID, parentCgroup, err := parseKubeCgroupParent(tt.args.parent)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseKubeCgroupParent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if podUID != tt.podUID {
				t.Errorf("parseKubeCgroupParent() got = %v, want %v", podUID, tt.podUID)
			}
			if containerID != tt.containerID {
				t.Errorf("parseKubeCgroupParent() got1 = %v, want %v", containerID, tt.containerID)
			}
			if parentCgroup != tt.parentCgroup {
				t.Errorf("parseKubeCgroupParent() got2 = %v, want %v", parentCgroup, tt.parentCgroup)
			}
		})
	}
}
