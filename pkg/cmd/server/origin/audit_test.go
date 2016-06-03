package origin

import (
	"bufio"
	"net"
	"net/http"
	"reflect"
	"testing"
)

type simpleResponseWriter struct {
	http.ResponseWriter
}

func (*simpleResponseWriter) WriteHeader(code int) {}

type fancyResponseWriter struct {
	simpleResponseWriter
}

func (*fancyResponseWriter) CloseNotify() <-chan bool { return nil }

func (*fancyResponseWriter) Flush() {}

func (*fancyResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) { return nil, nil, nil }

func TestConstructResponseWriter(t *testing.T) {
	actual := constructResponseWriter(&simpleResponseWriter{}, "")
	switch v := actual.(type) {
	case *auditResponseWriter:
		break
	default:
		t.Errorf("Expected auditResponseWriter, got %v", reflect.TypeOf(v))
	}

	actual = constructResponseWriter(&fancyResponseWriter{}, "")
	switch v := actual.(type) {
	case *fancyResponseWriterDelegator:
		break
	default:
		t.Errorf("Expected fancyResponseWriterDelegator, got %v", reflect.TypeOf(v))
	}
}
