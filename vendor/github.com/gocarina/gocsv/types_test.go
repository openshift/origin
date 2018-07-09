package gocsv

import (
	"reflect"
	"testing"
)

type sampleTypeUnmarshaller struct {
	val string
}

func (s *sampleTypeUnmarshaller) UnmarshalCSV(val string) error {
	s.val = val
	return nil
}

func (s sampleTypeUnmarshaller) MarshalCSV() (string, error) {
	return s.val, nil
}

type sampleTextUnmarshaller struct {
	val []byte
}

func (s *sampleTextUnmarshaller) UnmarshalText(text []byte) error {
	s.val = text
	return nil
}

func (s sampleTextUnmarshaller) MarshalText() ([]byte, error) {
	return s.val, nil
}

type sampleStringer string

func (s sampleStringer) String() string {
	return string(s)
}

func Benchmark_unmarshall_TypeUnmarshaller(b *testing.B) {
	sample := sampleTypeUnmarshaller{}
	val := reflect.ValueOf(&sample)
	for n := 0; n < b.N; n++ {
		if err := unmarshall(val, "foo"); err != nil {
			b.Fatalf("unmarshall error: %s", err.Error())
		}
	}
}

func Benchmark_unmarshall_TextUnmarshaller(b *testing.B) {
	sample := sampleTextUnmarshaller{}
	val := reflect.ValueOf(&sample)
	for n := 0; n < b.N; n++ {
		if err := unmarshall(val, "foo"); err != nil {
			b.Fatalf("unmarshall error: %s", err.Error())
		}
	}
}

func Benchmark_marshall_TypeMarshaller(b *testing.B) {
	sample := sampleTypeUnmarshaller{"foo"}
	val := reflect.ValueOf(&sample)
	for n := 0; n < b.N; n++ {
		_, err := marshall(val)
		if err != nil {
			b.Fatalf("marshall error: %s", err.Error())
		}
	}
}

func Benchmark_marshall_TextMarshaller(b *testing.B) {
	sample := sampleTextUnmarshaller{[]byte("foo")}
	val := reflect.ValueOf(&sample)
	for n := 0; n < b.N; n++ {
		_, err := marshall(val)
		if err != nil {
			b.Fatalf("marshall error: %s", err.Error())
		}
	}
}

func Benchmark_marshall_Stringer(b *testing.B) {
	sample := sampleStringer("foo")
	val := reflect.ValueOf(&sample)
	for n := 0; n < b.N; n++ {
		_, err := marshall(val)
		if err != nil {
			b.Fatalf("marshall error: %s", err.Error())
		}
	}
}
