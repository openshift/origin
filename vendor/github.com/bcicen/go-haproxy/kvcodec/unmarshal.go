package kvcodec

import (
	"bufio"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

const (
	tagLabel = "kv"
	kvDelim  = ":"
)

type structFields map[string]fieldMeta

type fieldMeta struct {
	Key        string
	Name       string
	OmitAlways bool
	OmitEmpty  bool
}

func newfieldMeta(field reflect.StructField) (meta fieldMeta) {
	meta = fieldMeta{
		Name: field.Name,
	}
	fieldTags := strings.Split(field.Tag.Get(tagLabel), ",")
	for _, tag := range fieldTags {
		if tag == "-" {
			meta.OmitAlways = true
			return meta
		}
		if tag == "omitempty" {
			meta.OmitEmpty = true
		} else if tag != "" {
			meta.Key = tag
		} else {
			meta.Key = field.Name
		}
	}
	return meta
}

var err error
var structMap = make(map[reflect.Type]structFields)
var structMapMutex sync.RWMutex

func getStructFields(rType reflect.Type) (structFields, error) {
	structMapMutex.RLock()
	stInfo, ok := structMap[rType]
	if !ok {
		stInfo, err = newStructFields(rType)
		if err != nil {
			return nil, err
		}
		structMap[rType] = stInfo
	}
	structMapMutex.RUnlock()
	return stInfo, nil
}

func newStructFields(rType reflect.Type) (structFields, error) {
	fieldsCount := rType.NumField()
	fieldMap := make(structFields)

	for i := 0; i < fieldsCount; i++ {
		field := rType.Field(i)
		meta := newfieldMeta(field)

		if field.PkgPath != "" {
			continue
		}

		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			return nil, fmt.Errorf("embedded structs not supported")
		}

		if !meta.OmitAlways {
			fieldMap[meta.Key] = meta
		}
	}

	return fieldMap, nil
}

func trim(s string) string {
	re := regexp.MustCompile("(\\S+|\\S+)")
	return strings.Join(re.FindAllString(s, -1), " ")
}

func Unmarshal(in io.Reader, out interface{}) error {
	outValue := reflect.ValueOf(out)
	if outValue.Kind() == reflect.Ptr {
		outValue = outValue.Elem()
	}

	fields, err := getStructFields(outValue.Type())
	if fields == nil {
		panic(err)
	}
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), kvDelim) {
			s := strings.Split(scanner.Text(), kvDelim)
			k, v := trim(s[0]), trim(s[1])
			if meta, ok := fields[k]; ok {
				field := outValue.FieldByName(meta.Name)
				err = setField(field, v)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
