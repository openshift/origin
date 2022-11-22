package ginkgo

import (
	"encoding/json"
	"fmt"
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
	fmt.Fprintf(opt.Out, "%s\n", data)

	//fmt.Printf("%#v\n", tests[len(tests)-1])
	//fmt.Printf("%v\n", tests[len(tests)-1].spec.Summary("").ComponentCodeLocations)
	//fmt.Printf("%#v\n", tests[0].spec.Summary("").ComponentCodeLocations)
	return nil
}
