package builder

import (
	"testing"
)

func TestReplaceValidCmd(t *testing.T) {
	tests := []struct {
		name           string
		cmd            string
		replaceStr     string
		fileData       []byte
		expectedOutput string
		expectedErr    error
	}{
		{
			name:           "from-replacement",
			cmd:            "from",
			replaceStr:     "FROM other/image",
			fileData:       []byte(dockerFile),
			expectedOutput: expectedFROM,
			expectedErr:    nil,
		},
		{
			name:           "run-replacement",
			cmd:            "run",
			replaceStr:     "This test kind-of-fails before string replacement so this string won't be used",
			fileData:       []byte(dockerFile),
			expectedOutput: dockerFile,
			expectedErr:    nil,
		},
		{
			name:           "invalid-dockerfile-cmd",
			cmd:            "blabla",
			replaceStr:     "This test fails at start so this string won't be used",
			fileData:       []byte(dockerFile),
			expectedOutput: "",
			expectedErr:    invalidCmdErr,
		},
		{
			name:           "no-cmd-in-dockerfile",
			cmd:            "cmd",
			replaceStr:     "CMD runme.sh",
			fileData:       []byte(dockerFile),
			expectedOutput: dockerFile,
			expectedErr:    nil,
		},
	}

	for _, test := range tests {
		out, err := replaceValidCmd(test.cmd, test.replaceStr, test.fileData)
		if err != test.expectedErr {
			t.Errorf("%s: Unexpected error: Expected %v, got %v", test.name, test.expectedErr, err)
		}
		if out != test.expectedOutput {
			t.Errorf("%s: Unexpected output: Expected %s, got %s", test.name, test.expectedOutput, out)
		}
	}
}

const dockerFile = `
FROM openshift/origin-base
FROM candidate

RUN mkdir -p /var/lib/openshift

ADD bin/openshift        /usr/bin/openshift
RUN ln -s /usr/bin/openshift /usr/bin/osc && \

ENV HOME /root
WORKDIR /var/lib/openshift
ENTRYPOINT ["/usr/bin/openshift"]
`

const expectedFROM = `
FROM openshift/origin-base
FROM other/image

RUN mkdir -p /var/lib/openshift

ADD bin/openshift        /usr/bin/openshift
RUN ln -s /usr/bin/openshift /usr/bin/osc && \

ENV HOME /root
WORKDIR /var/lib/openshift
ENTRYPOINT ["/usr/bin/openshift"]
`
