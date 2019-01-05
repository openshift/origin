package app

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/encoding"
	"github.com/mesos/mesos-go/api/v1/lib/encoding/codecs"
)

var (
	errZeroLengthLabelKey = errors.New("zero-length label key")
)

type URL struct{ url.URL }

func (u *URL) Set(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	u.URL = *parsed
	return nil
}

type codec struct{ encoding.Codec }

func (c *codec) Set(value string) error {
	v := strings.ToLower(value)
	for _, codec := range codecs.ByMediaType {
		if v == codec.Name {
			c.Codec = codec
			return nil
		}
	}
	return fmt.Errorf("bad codec %q", value)
}

type Labels []mesos.Label

func (labels *Labels) Set(value string) error {
	set := func(k, v string) {
		var val *string
		if v != "" {
			val = &v
		}
		*labels = append(*labels, mesos.Label{
			Key:   k,
			Value: val,
		})
	}
	e := strings.IndexRune(value, '=')
	c := strings.IndexRune(value, ':')
	if e != -1 && e < c {
		if e == 0 {
			return errZeroLengthLabelKey
		}
		set(value[:e], value[e+1:])
	} else if c != -1 && c < e {
		if c == 0 {
			return errZeroLengthLabelKey
		}
		set(value[:c], value[c+1:])
	} else if e != -1 {
		if e == 0 {
			return errZeroLengthLabelKey
		}
		set(value[:e], value[e+1:])
	} else if c != -1 {
		if c == 0 {
			return errZeroLengthLabelKey
		}
		set(value[:c], value[c+1:])
	} else if value != "" {
		set(value, "")
	}
	return nil
}

func (labels Labels) String() string {
	// super inefficient, but it's only for occassional debugging
	s := ""
	valueString := func(v *string) string {
		if v == nil {
			return ""
		}
		return ":" + *v
	}
	for _, x := range labels {
		if s != "" {
			s += ","
		}
		s += x.Key + valueString(x.Value)
	}
	return s
}
