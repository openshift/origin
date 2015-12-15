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
	containerId := readNetClsCGroup(buffer)

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
	containerId := readNetClsCGroup(buffer)

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
