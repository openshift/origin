package ginkgo

import (
	"encoding/json"
	"fmt"
	"os"
)

func (opt *Options) ListTests() error {
	tests, err := testsForSuite()
	if err != nil {
		return err
	}

	var serializableTests []test
	for _, t := range tests {
		newtest := test{t.name, t.spec.CodeLocations()}
		serializableTests = append(serializableTests, newtest)
		//fmt.Printf("%#v\n", newtest)
		//serializableTests = append(serializableTests, test{t.name, t.location.FileName})
	}
	data, err := json.Marshal(serializableTests)
	if err != nil {
		return err
	}
	if len(opt.TestListPath) == 0 {
		fmt.Fprintf(opt.Out, "%s\n", data)
		return nil
	}

	err = os.WriteFile(opt.TestListPath, data, 0644)
	return err
}
