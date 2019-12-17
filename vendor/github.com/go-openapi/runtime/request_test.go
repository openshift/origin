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
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

type eofReader struct{}

func (e *eofReader) Read(d []byte) (int, error) {
	return 0, io.EOF
}

func closeReader(rdr io.Reader) *closeCounting {
	return &closeCounting{
		rdr: rdr,
	}
}

type closeCounting struct {
	rdr    io.Reader
	closed int
}

func (c *closeCounting) Read(d []byte) (int, error) {
	return c.rdr.Read(d)
}

func (c *closeCounting) Close() error {
	c.closed++
	if cr, ok := c.rdr.(io.ReadCloser); ok {
		return cr.Close()
	}
	return nil
}

type countingBufioReader struct {
	buffereds int
	peeks     int
	reads     int

	br interface {
		Buffered() int
		Peek(int) ([]byte, error)
		Read([]byte) (int, error)
	}
}

func (c *countingBufioReader) Buffered() int {
	c.buffereds++
	return c.br.Buffered()
}

func (c *countingBufioReader) Peek(v int) ([]byte, error) {
	c.peeks++
	return c.br.Peek(v)
}

func (c *countingBufioReader) Read(p []byte) (int, error) {
	c.reads++
	return c.br.Read(p)
}

func TestPeekingReader(t *testing.T) {
	// just passes to original reader when nothing called
	exp1 := []byte("original")
	pr1 := newPeekingReader(closeReader(bytes.NewReader(exp1)))
	b1, err := ioutil.ReadAll(pr1)
	if assert.NoError(t, err) {
		assert.Equal(t, exp1, b1)
	}

	// uses actual when there was some buffering
	exp2 := []byte("actual")
	pr2 := newPeekingReader(closeReader(bytes.NewReader(exp2)))
	peeked, err := pr2.underlying.Peek(1)
	require.NoError(t, err)
	require.Equal(t, "a", string(peeked))
	b2, err := ioutil.ReadAll(pr2)
	if assert.NoError(t, err) {
		assert.Equal(t, string(exp2), string(b2))
	}

	// passes close call through to original reader
	cr := closeReader(closeReader(bytes.NewReader(exp2)))
	pr3 := newPeekingReader(cr)
	require.NoError(t, pr3.Close())
	require.Equal(t, 1, cr.closed)

	// returns false when the stream is empty
	pr4 := newPeekingReader(closeReader(&eofReader{}))
	require.False(t, pr4.HasContent())

	// returns true when the stream has content
	rdr := closeReader(strings.NewReader("hello"))
	pr := newPeekingReader(rdr)
	cbr := &countingBufioReader{
		br: bufio.NewReader(rdr),
	}
	pr.underlying = cbr

	require.True(t, pr.HasContent())
	require.Equal(t, 1, cbr.buffereds)
	require.Equal(t, 1, cbr.peeks)
	require.Equal(t, 0, cbr.reads)
	require.True(t, pr.HasContent())
	require.Equal(t, 2, cbr.buffereds)
	require.Equal(t, 1, cbr.peeks)
	require.Equal(t, 0, cbr.reads)

	b, err := ioutil.ReadAll(pr)
	require.NoError(t, err)
	require.Equal(t, "hello", string(b))
	require.Equal(t, 2, cbr.buffereds)
	require.Equal(t, 1, cbr.peeks)
	require.Equal(t, 2, cbr.reads)
	require.Equal(t, 0, cbr.br.Buffered())
}

func TestJSONRequest(t *testing.T) {
	req, err := JSONRequest("GET", "/swagger.json", nil)
	assert.NoError(t, err)
	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, JSONMime, req.Header.Get(HeaderContentType))
	assert.Equal(t, JSONMime, req.Header.Get(HeaderAccept))

	req, err = JSONRequest("GET", "%2", nil)
	assert.Error(t, err)
	assert.Nil(t, req)
}

//func TestCanHaveBody(t *testing.T) {
//assert.True(t, CanHaveBody("put"))
//assert.True(t, CanHaveBody("post"))
//assert.True(t, CanHaveBody("patch"))
//assert.True(t, CanHaveBody("delete"))
//assert.False(t, CanHaveBody(""))
//assert.False(t, CanHaveBody("get"))
//assert.False(t, CanHaveBody("options"))
//assert.False(t, CanHaveBody("head"))
//assert.False(t, CanHaveBody("invalid"))
//}

func TestReadSingle(t *testing.T) {
	values := url.Values(make(map[string][]string))
	values.Add("something", "the thing")
	assert.Equal(t, "the thing", ReadSingleValue(tv(values), "something"))
	assert.Empty(t, ReadSingleValue(tv(values), "notthere"))
}

func TestReadCollection(t *testing.T) {
	values := url.Values(make(map[string][]string))
	values.Add("something", "value1,value2")
	assert.Equal(t, []string{"value1", "value2"}, ReadCollectionValue(tv(values), "something", "csv"))
	assert.Empty(t, ReadCollectionValue(tv(values), "notthere", ""))
}

type tv map[string][]string

func (v tv) GetOK(key string) (value []string, hasKey bool, hasValue bool) {
	value, hasKey = v[key]
	if !hasKey {
		return
	}
	if len(value) == 0 {
		return
	}
	hasValue = true
	return

}
