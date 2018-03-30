package storageos

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/storageos/go-api/types"
)

func TestCPHealth(t *testing.T) {
	data := `{
  "submodules": {
    "kv": {
      "status": "alive",
      "message": "",
      "updatedAt": "2017-08-07T12:54:29.304265848Z",
      "changedAt": "2017-08-07T12:46:43.53977923Z"
    },
    "kv_write": {
      "status": "alive",
      "message": "",
      "updatedAt": "2017-08-07T12:54:29.304266573Z",
      "changedAt": "0001-01-01T00:00:00Z"
    },
    "nats": {
      "status": "alive",
      "message": "",
      "updatedAt": "2017-08-07T12:54:29.304265286Z",
      "changedAt": "0001-01-01T00:00:00Z"
    },
    "scheduler": {
      "status": "alive",
      "message": "",
      "updatedAt": "2017-08-07T12:54:29.304266169Z",
      "changedAt": "2017-08-07T12:47:03.207074102Z"
    }
  }
}`

	var expected types.CPHealthStatus
	if err := json.Unmarshal([]byte(data), &expected); err != nil {
		t.Fatal(err)
	}

	client := newTestClient(&FakeRoundTripper{message: data, status: http.StatusOK})
	cpHealth, err := client.CPHealth(context.Background(), "someHostname")
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(*cpHealth, expected) {
		t.Errorf("Wrong return value.\nWant %#v.\nGot %#v.", expected, *cpHealth)
	}
}

func TestDPHealth(t *testing.T) {
	data := `{
  "submodules": {
    "directfs-client": {
      "status": "alive",
      "message": "",
      "updatedAt": "2017-08-07T12:55:57.179942558Z",
      "changedAt": "2017-08-07T12:46:58.975050385Z"
    },
    "directfs-server": {
      "status": "alive",
      "message": "",
      "updatedAt": "2017-08-07T12:55:57.179939863Z",
      "changedAt": "2017-08-07T12:46:58.975047436Z"
    },
    "director": {
      "status": "alive",
      "message": "",
      "updatedAt": "2017-08-07T12:55:57.17994324Z",
      "changedAt": "2017-08-07T12:46:58.975051141Z"
    },
    "filesystem-driver": {
      "status": "alive",
      "message": "",
      "updatedAt": "2017-08-07T12:55:57.179944197Z",
      "changedAt": "2017-08-07T12:47:09.418689273Z"
    },
    "fs": {
      "status": "alive",
      "message": "",
      "updatedAt": "2017-08-07T12:55:57.179945193Z",
      "changedAt": "2017-08-07T12:47:09.418690078Z"
    }
  }
}
`
	var expected types.DPHealthStatus
	if err := json.Unmarshal([]byte(data), &expected); err != nil {
		t.Fatal(err)
	}

	client := newTestClient(&FakeRoundTripper{message: data, status: http.StatusOK})
	dpHealth, err := client.DPHealth(context.Background(), "someHostname")
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(*dpHealth, expected) {
		t.Errorf("Wrong return value.\nWant %#v.\nGot %#v.", expected, *dpHealth)
	}
}
