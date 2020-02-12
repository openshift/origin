package json

import (
	"reflect"
	"strconv"
	"unsafe"

	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
)

func init() {
	// TODO: drop in 4.0
	jsoniter.RegisterExtension(&coercerExtension{})
}

type coercerExtension struct {
	jsoniter.DummyExtension
}

func (*coercerExtension) DecorateDecoder(typ reflect2.Type, decoder jsoniter.ValDecoder) jsoniter.ValDecoder {
	switch typ.Kind() {
	case reflect.Int8, reflect.Uint8, reflect.Int16, reflect.Uint16, reflect.Int32, reflect.Uint32, reflect.Int64, reflect.Uint64, reflect.Int, reflect.Uint:
		return &intCoercer{typ.Kind(), decoder}
	case reflect.Slice:
		return &sliceCoercer{decoder}
	default:
		return decoder
	}
}

// intCoercer attempts to parse strings to integer fields. drop in 4.0.
type intCoercer struct {
	kind reflect.Kind
	jsoniter.ValDecoder
}

func (c *intCoercer) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	switch iter.WhatIsNext() {
	case jsoniter.StringValue:
		var str string
		iter.ReadVal(&str)
		switch c.kind {
		case reflect.Int8:
			if i, err := strconv.ParseInt(str, 10, 8); err == nil {
				*(*int8)(ptr) = int8(i)
			} else {
				iter.ReportError("read int8", "unexpected character: \"")
			}
		case reflect.Int16:
			if i, err := strconv.ParseInt(str, 10, 16); err == nil {
				*(*int16)(ptr) = int16(i)
			} else {
				iter.ReportError("read int16", "unexpected character: \"")
			}
		case reflect.Int32:
			if i, err := strconv.ParseInt(str, 10, 32); err == nil {
				*(*int32)(ptr) = int32(i)
			} else {
				iter.ReportError("read int32", "unexpected character: \"")
			}
		case reflect.Int64:
			if i, err := strconv.ParseInt(str, 10, 64); err == nil {
				*(*int64)(ptr) = int64(i)
			} else {
				iter.ReportError("read int64", "unexpected character: \"")
			}
		case reflect.Int:
			if i, err := strconv.ParseInt(str, 10, 64); err == nil {
				*(*int)(ptr) = int(i)
			} else {
				iter.ReportError("read int", "unexpected character: \"")
			}

		case reflect.Uint8:
			if i, err := strconv.ParseUint(str, 10, 8); err == nil {
				*(*uint8)(ptr) = uint8(i)
			} else {
				iter.ReportError("read uint8", "unexpected character: \"")
			}
		case reflect.Uint16:
			if i, err := strconv.ParseUint(str, 10, 16); err == nil {
				*(*uint16)(ptr) = uint16(i)
			} else {
				iter.ReportError("read uint16", "unexpected character: \"")
			}
		case reflect.Uint32:
			if i, err := strconv.ParseUint(str, 10, 32); err == nil {
				*(*uint32)(ptr) = uint32(i)
			} else {
				iter.ReportError("read uint32", "unexpected character: \"")
			}
		case reflect.Uint64:
			if i, err := strconv.ParseUint(str, 10, 64); err == nil {
				*(*uint64)(ptr) = uint64(i)
			} else {
				iter.ReportError("read uint16", "unexpected character: \"")
			}
		case reflect.Uint:
			if i, err := strconv.ParseUint(str, 10, 64); err == nil {
				*(*uint)(ptr) = uint(i)
			} else {
				iter.ReportError("read uint", "unexpected character: \"")
			}

		default:
			iter.ReportError("read number", "unexpected character: \"")
		}
	default:
		c.ValDecoder.Decode(ptr, iter)
	}
}

// sliceCoercer tolerates empty object values (`{}`) for slice fields. drop in 4.0.
type sliceCoercer struct {
	jsoniter.ValDecoder
}

func (c *sliceCoercer) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	switch iter.WhatIsNext() {
	case jsoniter.ObjectValue:
		var obj map[string]interface{}
		iter.ReadVal(&obj)
		if len(obj) > 0 {
			iter.ReportError("decode slice", "expect [ or n, but found {")
		}
	default:
		c.ValDecoder.Decode(ptr, iter)
	}
}
