package dockerfile

import "testing"

// TestEnv tests calling Env with multiple inputs.
func TestEnv(t *testing.T) {
	testCases := []struct {
		in   []KeyValue
		want string
	}{
		{
			in:   nil,
			want: `ENV`,
		},
		{
			in:   []KeyValue{},
			want: `ENV`,
		},
		{
			in: []KeyValue{
				{"", ""},
				{"", "ABC"},
				{"ABC", ""},
			},
			want: `ENV ""="" ""="ABC" "ABC"=""`,
		},
		{
			in: []KeyValue{
				{"GOPATH", "/go"},
				{"MSG", "Hello World!"},
			},
			want: `ENV "GOPATH"="/go" "MSG"="Hello World!"`,
		},
		{
			in: []KeyValue{
				{"PATH", "/bin"},
				{"GOPATH", "/go"},
				{"PATH", "$GOPATH/bin:$PATH"},
			},
			want: `ENV "PATH"="/bin" "GOPATH"="/go" "PATH"="$GOPATH/bin:$PATH"`,
		},
		{
			in: []KeyValue{
				{"你好", "我会说汉语"},
			},
			want: `ENV "你好"="我会说汉语"`,
		},
		{
			// This tests handling an string encoding edge case.
			// Example input taken from Docker parser's test suite.
			in: []KeyValue{
				{"☃", "'\" \\ / \b \f \n \r \t \x00"},
			},
			want: `ENV "☃"="'\" \\ / \u0008 \u000c \n \r \t \u0000"`,
		},
	}
	for _, tc := range testCases {
		got, err := Env(tc.in)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("Env(%v) = %q; want %q", tc.in, got, tc.want)
		}
	}
}
