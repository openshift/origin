package schema

import (
	"encoding/json"
	"errors"
)

// UnmarshalJSON initializes the primitive type from
// a JSON string.
func (t *PrimitiveType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	x, err := primitiveFromString(string(data))
	if err != nil {
		return err
	}
	*t = x
	return nil
}

func primitiveFromString(s string) (t PrimitiveType, err error) {
	switch s {
	case "null":
		t = NullType
	case "integer":
		t = IntegerType
	case "string":
		t = StringType
	case "object":
		t = ObjectType
	case "array":
		t = ArrayType
	case "boolean":
		t = BooleanType
	case "number":
		t = NumberType
	default:
		err = errors.New("unknown primitive type: " + s)
	}
	return
}

// String returns the string representation of this primitive type
func (t PrimitiveType) String() string {
	var v string
	switch t {
	case NullType:
		v = "null"
	case IntegerType:
		v = "integer"
	case StringType:
		v = "string"
	case ObjectType:
		v = "object"
	case ArrayType:
		v = "array"
	case BooleanType:
		v = "boolean"
	case NumberType:
		v = "number"
	default:
		v = "<invalid>"
	}
	return v
}

// MarshalJSON seriealises the primitive type into a JSON string
func (t PrimitiveType) MarshalJSON() ([]byte, error) {
	switch t {
	case NullType, IntegerType, StringType, ObjectType, ArrayType, BooleanType, NumberType:
		return json.Marshal(t.String())
	default:
		return nil, errors.New("unknown primitive type")
	}
}

// UnmarshalJSON initializes the list of primitive types
func (pt *PrimitiveTypes) UnmarshalJSON(data []byte) error {
	if data[0] != '[' {
		var t PrimitiveType
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}

		*pt = PrimitiveTypes{t}
		return nil
	}

	var list []PrimitiveType
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	*pt = PrimitiveTypes(list)
	return nil
}

// Bool returns the underlying boolean value for the
// primitive boolean type
func (b Bool) Bool() bool {
	if b.Initialized {
		return b.Val
	}
	return b.Default
}

// Contains returns true if the list of primitive types
// contains `p`
func (pt PrimitiveTypes) Contains(p PrimitiveType) bool {
	for _, v := range pt {
		if p == v {
			return true
		}
	}
	return false
}

// Len returns the length of the list of primitive types
func (pt PrimitiveTypes) Len() int {
	return len(pt)
}

// Less returns true if the i-th element in the list is
// listed before the j-th element.
func (pt PrimitiveTypes) Less(i, j int) bool {
	return pt[i] < pt[j]
}

// Swap swaps the elements in positions i and j
func (pt PrimitiveTypes) Swap(i, j int) {
	pt[i], pt[j] = pt[j], pt[i]
}
