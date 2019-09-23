package framing_test

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"

	. "github.com/mesos/mesos-go/api/v1/lib/encoding/framing"
)

func TestNewDecoder(t *testing.T) {
	var (
		byteCopy = UnmarshalFunc(func(b []byte, m interface{}) error {
			if m == nil {
				return errors.New("unmarshal target may not be nil")
			}
			v, ok := m.(*[]byte)
			if !ok {
				return fmt.Errorf("expected *[]byte instead of %T", m)
			}
			if v == nil {
				return errors.New("target *[]byte may not be nil")
			}
			*v = append((*v)[:0], b...)
			return nil
		})
		fakeError        = errors.New("fake unmarshal error")
		errorUnmarshaler = UnmarshalFunc(func(_ []byte, _ interface{}) error {
			return fakeError
		})
	)
	for ti, tc := range []struct {
		r        Reader
		uf       UnmarshalFunc
		wants    [][]byte
		wantsErr error
	}{
		{errorReader(ErrorBadSize), byteCopy, nil, ErrorBadSize},
		{tokenReader("james"), byteCopy, frames("james"), nil},
		{tokenReader("james", "foo"), byteCopy, frames("james", "foo"), nil},
		{tokenReader("", "foo"), byteCopy, frames("", "foo"), nil},
		{tokenReader("foo", ""), byteCopy, frames("foo", ""), nil},
		{tokenReader(""), byteCopy, frames(""), nil},
		{tokenReader(), byteCopy, frames(), io.EOF},
		{tokenReader("james"), errorUnmarshaler, nil, fakeError},
	} {
		t.Run(fmt.Sprintf("test case %d", ti), func(t *testing.T) {
			if (tc.wants == nil) != (tc.wantsErr != nil) {
				t.Fatalf("invalid test case: cannot expect both data and an error")
			}
			var (
				f   [][]byte
				d   = NewDecoder(tc.r, tc.uf)
				err error
			)
			for err == nil {
				var buf []byte
				err = d.Decode(&buf)
				if err == io.EOF {
					break
				}
				if err == nil {
					f = append(f, buf)
				}
				if err != tc.wantsErr {
					t.Errorf("expected error %q instead of %q", tc.wantsErr, err)
				}
			}
			if !reflect.DeepEqual(f, tc.wants) {
				t.Errorf("expected %#v instead of %#v", tc.wants, f)
			}
		})
	}
}

func tokenReader(s ...string) ReaderFunc {
	if len(s) == 0 {
		return EOFReaderFunc
	}
	ch := make(chan []byte, len(s))
	for i := range s {
		ch <- ([]byte)(s[i])
	}
	return func() ([]byte, error) {
		select {
		case b := <-ch:
			return b, nil
		default:
			return nil, io.EOF
		}
	}
}

func errorReader(err error) ReaderFunc {
	return func() ([]byte, error) { return nil, err }
}

func frames(s ...string) (f [][]byte) {
	if len(s) == 0 {
		return nil
	}
	f = make([][]byte, 0, len(s))
	for i := range s {
		// converting to/from []byte and string for empty string isn't a perfectly symmetrical
		// operation. fix it up here with a quick length check.
		if len(s[i]) == 0 {
			f = append(f, nil)
			continue
		}
		f = append(f, ([]byte)(s[i]))
	}
	return
}
