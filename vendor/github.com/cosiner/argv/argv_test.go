package argv

import (
	"os"
	"reflect"
	"testing"
)

func TestArgv(t *testing.T) {
	type testCase struct {
		Input    string
		Sections [][]string
		Error    error
	}
	cases := []testCase{
		{
			Input: " a | a|a |a`ls ~/``ls /` ",
			Sections: [][]string{
				{"a"},
				{"a"},
				{"a"},
				{"als ~/ls /"},
			},
		},
		{
			Input: "aaa |",
			Error: ErrInvalidSyntax,
		},
		{
			Input: "aaa | | aa",
			Error: ErrInvalidSyntax,
		},
		{
			Input: " | aa",
			Error: ErrInvalidSyntax,
		},
		{
			Input: `aa"aaa`,
			Error: ErrInvalidSyntax,
		},
	}
	for i, c := range cases {
		gots, err := Argv([]rune(c.Input), nil, nil)
		if err != c.Error {
			t.Errorf("test failed: %d, expect error:%s, but got %s", i, c.Error, err)

		}
		if err != nil {
			continue
		}

		if !reflect.DeepEqual(gots, c.Sections) {
			t.Errorf("parse failed %d, expect: %v, got %v", i, c.Sections, gots)
		}
	}
}

func TestRun(t *testing.T) {
	output, err := Run([]rune("echo / | wc -l"), ParseEnv(os.Environ()))
	t.Log(string(output), err)
}
