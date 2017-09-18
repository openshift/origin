package builder

import (
	"bytes"
	"strings"
	"testing"
)

func TestCGroups_CentOS7_Docker1_7(t *testing.T) {
	example := `10:hugetlb:/
9:perf_event:/docker-daemon
8:blkio:/system.slice/docker-5617ed7e7e487d2c4dd2e013e361109b4eceabfe3fa8c7aea9e37498b1aed5fa.scope
7:net_cls:/
6:freezer:/system.slice/docker-5617ed7e7e487d2c4dd2e013e361109b4eceabfe3fa8c7aea9e37498b1aed5fa.scope
5:devices:/system.slice/docker-5617ed7e7e487d2c4dd2e013e361109b4eceabfe3fa8c7aea9e37498b1aed5fa.scope
4:memory:/system.slice/docker-5617ed7e7e487d2c4dd2e013e361109b4eceabfe3fa8c7aea9e37498b1aed5fa.scope
3:cpuacct,cpu:/system.slice/docker-5617ed7e7e487d2c4dd2e013e361109b4eceabfe3fa8c7aea9e37498b1aed5fa.scope
2:cpuset:/system.slice/docker-5617ed7e7e487d2c4dd2e013e361109b4eceabfe3fa8c7aea9e37498b1aed5fa.scope
1:name=systemd:/system.slice/docker.service
`
	buffer := bytes.NewBufferString(example)
	containerId, _ := readNetClsCGroup(buffer)

	if containerId != "5617ed7e7e487d2c4dd2e013e361109b4eceabfe3fa8c7aea9e37498b1aed5fa" {
		t.Errorf("got %s, expected 5617ed7e7e487d2c4dd2e013e361109b4eceabfe3fa8c7aea9e37498b1aed5fa", containerId)
	}
}

func TestCGroups_Ubuntu_Docker1_9(t *testing.T) {
	example := `11:hugetlb:/docker/bfea6eb2d60179355e370a5d277d496eb0fe75d9a5a47c267221e87dbbbbc93b
10:perf_event:/docker/bfea6eb2d60179355e370a5d277d496eb0fe75d9a5a47c267221e87dbbbbc93b
9:blkio:/docker/bfea6eb2d60179355e370a5d277d496eb0fe75d9a5a47c267221e87dbbbbc93b
8:freezer:/docker/bfea6eb2d60179355e370a5d277d496eb0fe75d9a5a47c267221e87dbbbbc93b
7:devices:/docker/bfea6eb2d60179355e370a5d277d496eb0fe75d9a5a47c267221e87dbbbbc93b
6:memory:/docker/bfea6eb2d60179355e370a5d277d496eb0fe75d9a5a47c267221e87dbbbbc93b
5:cpuacct:/docker/bfea6eb2d60179355e370a5d277d496eb0fe75d9a5a47c267221e87dbbbbc93b
4:cpu:/docker/bfea6eb2d60179355e370a5d277d496eb0fe75d9a5a47c267221e87dbbbbc93b
3:cpuset:/docker/bfea6eb2d60179355e370a5d277d496eb0fe75d9a5a47c267221e87dbbbbc93b
2:name=systemd:/`
	buffer := bytes.NewBufferString(example)
	containerId, _ := readNetClsCGroup(buffer)

	if containerId != "bfea6eb2d60179355e370a5d277d496eb0fe75d9a5a47c267221e87dbbbbc93b" {
		t.Errorf("got %s, expected bfea6eb2d60179355e370a5d277d496eb0fe75d9a5a47c267221e87dbbbbc93b", containerId)
	}
}

func TestMergeEnv(t *testing.T) {
	tests := []struct {
		oldEnv   []string
		newEnv   []string
		expected []string
	}{
		{
			oldEnv:   []string{"one=1", "two=2"},
			newEnv:   []string{"three=3", "four=4"},
			expected: []string{"one=1", "two=2", "three=3", "four=4"},
		},
		{
			oldEnv:   []string{"one=1", "two=2", "four=4"},
			newEnv:   []string{"three=3", "four=4=5=6"},
			expected: []string{"one=1", "two=2", "three=3", "four=4=5=6"},
		},
		{
			oldEnv:   []string{"one=1", "two=2", "three=3"},
			newEnv:   []string{"two=002", "four=4"},
			expected: []string{"one=1", "two=002", "three=3", "four=4"},
		},
		{
			oldEnv:   []string{"one=1", "=2"},
			newEnv:   []string{"=3", "two=2"},
			expected: []string{"one=1", "=3", "two=2"},
		},
		{
			oldEnv:   []string{"one=1", "two"},
			newEnv:   []string{"two=2", "three=3"},
			expected: []string{"one=1", "two=2", "three=3"},
		},
	}
	for _, tc := range tests {
		result := MergeEnv(tc.oldEnv, tc.newEnv)
		toCheck := map[string]struct{}{}
		for _, e := range tc.expected {
			toCheck[e] = struct{}{}
		}
		for _, e := range result {
			if _, exists := toCheck[e]; !exists {
				t.Errorf("old = %s, new = %s: %s not expected in result",
					strings.Join(tc.oldEnv, ","), strings.Join(tc.newEnv, ","), e)
				continue
			}
			delete(toCheck, e)
		}
		if len(toCheck) > 0 {
			t.Errorf("old = %s, new = %s: did not get expected values in result: %#v",
				strings.Join(tc.oldEnv, ","), strings.Join(tc.newEnv, ","), toCheck)
		}
	}
}

type testcase struct {
	name   string
	input  map[string]string
	expect string
	fail   bool
}

func TestCGroupParentExtraction(t *testing.T) {
	tcs := []testcase{
		{
			name: "systemd",
			input: map[string]string{
				"cpu":          "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"cpuacct":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"name=systemd": "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"net_prio":     "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"freezer":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"blkio":        "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"net_cls":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"memory":       "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"hugetlb":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"perf_event":   "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"cpuset":       "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"devices":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
				"pids":         "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344.scope",
			},
			expect: "kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice",
		},
		{
			name: "systemd-besteffortpod",
			input: map[string]string{
				"memory": "/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod8d369e32_521b_11e7_8df4_507b9d27b5d9.slice/docker-fbf6fe5e4effd80b6a9b3318dd0e5f538b9c4ba8918c174768720c83b338a41f.scope",
			},
			expect: "kubepods-besteffort-pod8d369e32_521b_11e7_8df4_507b9d27b5d9.slice",
		},
		{
			name: "nonsystemd-burstablepod",
			input: map[string]string{
				"memory": "/kubepods/burstable/podc4ab0636-521a-11e7-8eea-0e5e65642be0/9ea9361dc31b0e18f699497a5a78a010eb7bae3f9a2d2b5d3027b37bdaa4b334",
			},
			expect: "/kubepods/burstable/podc4ab0636-521a-11e7-8eea-0e5e65642be0",
		},
		{
			name: "non-systemd",
			input: map[string]string{
				"cpu":          "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"cpuacct":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"name=systemd": "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"net_prio":     "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"freezer":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"blkio":        "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"net_cls":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"memory":       "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"hugetlb":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"perf_event":   "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"cpuset":       "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"devices":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"pids":         "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
			},
			expect: "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice",
		},
		{
			name: "no-memory-entry",
			input: map[string]string{
				"cpu":          "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"cpuacct":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"name=systemd": "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"net_prio":     "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"freezer":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"blkio":        "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"net_cls":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"hugetlb":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"perf_event":   "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"cpuset":       "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"devices":      "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
				"pids":         "/kubepods.slice/kubepods-podd0d034ed_5204_11e7_9710_507b9d27b5d9.slice/docker-a91b1981b6a4ae463fccd273e3f8665fd911e9abcaa1af27de773afb17ec4344",
			},
			expect: "",
			fail:   true,
		},
		{
			name: "unparseable",
			input: map[string]string{
				"memory": "kubepods.slice",
			},
			expect: "",
			fail:   true,
		},
	}

	for _, tc := range tcs {
		parent, err := extractParentFromCgroupMap(tc.input)
		if err != nil && !tc.fail {
			t.Errorf("[%s] unexpected exception: %v", tc.name, err)
		}
		if tc.fail && err == nil {
			t.Errorf("[%s] expected failure, did not get one and got cgroup parent=%s", tc.name, parent)
		}
		if parent != tc.expect {
			t.Errorf("[%s] expected cgroup parent= %s, got %s", tc.name, tc.expect, parent)
		}
	}
}
