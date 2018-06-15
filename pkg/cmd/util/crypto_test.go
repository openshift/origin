package util

import (
	"io/ioutil"
	"testing"
)

func TestPrivateKeysFromPEM(t *testing.T) {
	data, err := ioutil.ReadFile("../../../test/testdata/router/default_pub_keys.pem")
	if err != nil {
		t.Fatal(err)
	}
	result, err := PrivateKeysFromPEM(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) == 0 {
		t.Fatalf("didn't extract results: %s", result)
	}
}
