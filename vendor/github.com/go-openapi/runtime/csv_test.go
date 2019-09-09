// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runtime

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http/httptest"
	"testing"
)

const consProdCSV = `name,country,age
John,US,19
Mike,US,20
`

type csvEmptyReader struct{}

func (r *csvEmptyReader) Read(d []byte) (int, error) {
	return 0, io.EOF
}

func TestCSVConsumer(t *testing.T) {
	cons := CSVConsumer()
	reader := bytes.NewBuffer([]byte(consProdCSV))

	outBuf := new(bytes.Buffer)
	err := cons.Consume(reader, outBuf)
	assert.NoError(t, err)
	assert.Equal(t, consProdCSV, outBuf.String())

	outBuf2 := new(bytes.Buffer)
	err = cons.Consume(nil, outBuf2)
	assert.Error(t, err)

	err = cons.Consume(reader, struct{}{})
	assert.Error(t, err)

	emptyOutBuf := new(bytes.Buffer)
	err = cons.Consume(&csvEmptyReader{}, emptyOutBuf)
	assert.NoError(t, err)
	assert.Equal(t, "", emptyOutBuf.String())
}

func TestCSVProducer(t *testing.T) {
	prod := CSVProducer()
	data := []byte(consProdCSV)

	rw := httptest.NewRecorder()
	err := prod.Produce(rw, data)
	assert.NoError(t, err)
	assert.Equal(t, consProdCSV, rw.Body.String())

	rw2 := httptest.NewRecorder()
	err = prod.Produce(rw2, struct{}{})
	assert.Error(t, err)

	err = prod.Produce(nil, data)
	assert.Error(t, err)
}
