package builder

import (
	"bytes"
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
