package gocsv

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// --------------------------------------------------------------------------
// Reflection helpers

type structInfo struct {
	Fields []fieldInfo
}

// fieldInfo is a struct field that should be mapped to a CSV column, or vice-versa
// Each IndexChain element before the last is the index of an the embedded struct field
// that defines Key as a tag
type fieldInfo struct {
	keys         []string
	omitEmpty    bool
	IndexChain   []int
	defaultValue string
	partial      bool
}

func (f fieldInfo) getFirstKey() string {
	return f.keys[0]
}

func (f fieldInfo) matchesKey(key string) bool {
	for _, k := range f.keys {
		if key == k || strings.TrimSpace(key) == k || (f.partial && strings.Contains(key, k)) || removeZeroWidthChars(key) == k {
			return true
		}
	}
	return false
}

// zwchs is Zero Width Characters map
var zwchs = map[rune]struct{}{
	'\u200B': {}, // zero width space (U+200B)
	'\uFEFF': {}, // zero width no-break space (U+FEFF)
	'\u200D': {}, // zero width joiner (U+200D)
	'\u200C': {}, // zero width non-joiner (U+200C)
}

func removeZeroWidthChars(s string) string {
	return strings.Map(func(r rune) rune {
		if _, ok := zwchs[r]; ok {
			return -1
		}
		return r
	}, s)
}

var structInfoCache sync.Map
var structMap = make(map[reflect.Type]*structInfo)
var structMapMutex sync.RWMutex

func getStructInfo(rType reflect.Type) *structInfo {
	stInfo, ok := structInfoCache.Load(rType)
	if ok {
		return stInfo.(*structInfo)
	}

	fieldsList := getFieldInfos(rType, []int{}, []string{})
	stInfo = &structInfo{fieldsList}
	structInfoCache.Store(rType, stInfo)

	return stInfo.(*structInfo)
}

func getFieldInfos(rType reflect.Type, parentIndexChain []int, parentKeys []string) []fieldInfo {
	fieldsCount := rType.NumField()
	fieldsList := make([]fieldInfo, 0, fieldsCount)
	for i := 0; i < fieldsCount; i++ {
		field := rType.Field(i)
		if field.PkgPath != "" {
			continue
		}

		var cpy = make([]int, len(parentIndexChain))
		copy(cpy, parentIndexChain)
		indexChain := append(cpy, i)

		var currFieldInfo *fieldInfo
		if !field.Anonymous {
			filteredTags := []string{}
			currFieldInfo, filteredTags = filterTags(TagName, indexChain, field)

			if len(filteredTags) == 1 && filteredTags[0] == "-" {
				// ignore nested structs with - tag
				continue
			} else if len(filteredTags) > 0 && filteredTags[0] != "" {
				currFieldInfo.keys = filteredTags
			} else {
				currFieldInfo.keys = []string{normalizeName(field.Name)}
			}

			if len(parentKeys) > 0 && currFieldInfo != nil {
				// create cartesian product of keys
				// eg: parent keys x field keys
				keys := make([]string, 0, len(parentKeys)*len(currFieldInfo.keys))
				for _, pkey := range parentKeys {
					for _, ckey := range currFieldInfo.keys {
						keys = append(keys, normalizeName(fmt.Sprintf("%s%s%s", pkey, FieldsCombiner, ckey)))
					}
					currFieldInfo.keys = keys
				}
			}
		}

		// handle struct
		fieldType := field.Type
		// if the field is a pointer, follow the pointer
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		// if the field is a struct, create a fieldInfo for each of its fields
		if fieldType.Kind() == reflect.Struct {
			// Structs that implement any of the text or CSV marshaling methods
			// should result in one value and not have their fields exposed
			if !(canMarshal(fieldType)) {
				// if the field is an embedded struct, pass along parent keys
				keys := parentKeys
				if currFieldInfo != nil {
					keys = currFieldInfo.keys
				}
				fieldsList = append(fieldsList, getFieldInfos(fieldType, indexChain, keys)...)
				continue
			}
		}

		// if the field is an embedded struct, ignore the csv tag
		if currFieldInfo == nil {
			continue
		}

		if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
			var arrayLength = -1
			// if the field is a slice or an array, see if it has a `csv[n]` tag
			if arrayTag, ok := field.Tag.Lookup(TagName + "[]"); ok {
				arrayLength, _ = strconv.Atoi(arrayTag)
			}

			// slices or arrays of Struct get special handling
			if field.Type.Elem().Kind() == reflect.Struct {
				fieldInfos := getFieldInfos(field.Type.Elem(), []int{}, []string{})

				// if no special csv[] tag was supplied, just include the field directly
				if arrayLength == -1 {
					fieldsList = append(fieldsList, *currFieldInfo)
				} else {
					// When the field is a slice/array of structs, create a fieldInfo for each index and each field
					for idx := 0; idx < arrayLength; idx++ {
						// copy index chain and append array index
						var cpy2 = make([]int, len(indexChain))
						copy(cpy2, indexChain)
						arrayIndexChain := append(cpy2, idx)
						for _, childFieldInfo := range fieldInfos {
							// copy array index chain and append array index
							var cpy3 = make([]int, len(arrayIndexChain))
							copy(cpy3, arrayIndexChain)

							arrayFieldInfo := fieldInfo{
								IndexChain:   append(cpy3, childFieldInfo.IndexChain...),
								omitEmpty:    childFieldInfo.omitEmpty,
								defaultValue: childFieldInfo.defaultValue,
								partial:      childFieldInfo.partial,
							}

							// create cartesian product of keys
							// eg: array field keys x struct field keys
							for _, akey := range currFieldInfo.keys {
								for _, fkey := range childFieldInfo.keys {
									arrayFieldInfo.keys = append(arrayFieldInfo.keys, normalizeName(fmt.Sprintf("%s[%d].%s", akey, idx, fkey)))
								}
							}

							fieldsList = append(fieldsList, arrayFieldInfo)
						}
					}
				}
			} else if arrayLength > 0 {
				// When the field is a slice/array of primitives, create a fieldInfo for each index
				for idx := 0; idx < arrayLength; idx++ {
					// copy index chain and append array index
					var cpy2 = make([]int, len(indexChain))
					copy(cpy2, indexChain)

					arrayFieldInfo := fieldInfo{
						IndexChain:   append(cpy2, idx),
						omitEmpty:    currFieldInfo.omitEmpty,
						defaultValue: currFieldInfo.defaultValue,
						partial:      currFieldInfo.partial,
					}

					for _, akey := range currFieldInfo.keys {
						arrayFieldInfo.keys = append(arrayFieldInfo.keys, normalizeName(fmt.Sprintf("%s[%d]", akey, idx)))
					}

					fieldsList = append(fieldsList, arrayFieldInfo)
				}
			} else {
				fieldsList = append(fieldsList, *currFieldInfo)
			}
		} else {
			fieldsList = append(fieldsList, *currFieldInfo)
		}
	}
	return fieldsList
}

func filterTags(tagName string, indexChain []int, field reflect.StructField) (*fieldInfo, []string) {
	currFieldInfo := fieldInfo{IndexChain: indexChain}

	fieldTag := field.Tag.Get(tagName)
	fieldTags := strings.Split(fieldTag, TagSeparator)

	filteredTags := []string{}
	for _, fieldTagEntry := range fieldTags {
		trimmedFieldTagEntry := strings.TrimSpace(fieldTagEntry) // handles cases like `csv:"foo, omitempty, default=test"`
		if trimmedFieldTagEntry == "omitempty" {
			currFieldInfo.omitEmpty = true
		} else if strings.HasPrefix(trimmedFieldTagEntry, "partial") {
			currFieldInfo.partial = true
		} else if strings.HasPrefix(trimmedFieldTagEntry, "default=") {
			currFieldInfo.defaultValue = strings.TrimPrefix(trimmedFieldTagEntry, "default=")
		} else {
			filteredTags = append(filteredTags, normalizeName(trimmedFieldTagEntry))
		}
	}

	return &currFieldInfo, filteredTags
}

func getConcreteContainerInnerType(in reflect.Type) (inInnerWasPointer bool, inInnerType reflect.Type) {
	inInnerType = in.Elem()
	inInnerWasPointer = false
	if inInnerType.Kind() == reflect.Ptr {
		inInnerWasPointer = true
		inInnerType = inInnerType.Elem()
	}
	return inInnerWasPointer, inInnerType
}

func getConcreteReflectValueAndType(in interface{}) (reflect.Value, reflect.Type) {
	value := reflect.ValueOf(in)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	return value, value.Type()
}

var errorInterface = reflect.TypeOf((*error)(nil)).Elem()

func isErrorType(outType reflect.Type) bool {
	if outType.Kind() != reflect.Interface {
		return false
	}

	return outType.Implements(errorInterface)
}
