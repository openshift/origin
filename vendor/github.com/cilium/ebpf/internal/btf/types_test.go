package btf

import "testing"

import "fmt"

func TestSizeof(t *testing.T) {
	testcases := []struct {
		size int
		typ  Type
	}{
		{1, &Int{Size: 1}},
		{4, &Enum{}},
		{0, &Array{Type: &Pointer{Target: Void{}}, Nelems: 0}},
		{12, &Array{Type: &Enum{}, Nelems: 3}},
	}

	for _, tc := range testcases {
		name := fmt.Sprint(tc.typ)
		t.Run(name, func(t *testing.T) {
			have, err := Sizeof(tc.typ)
			if err != nil {
				t.Fatal("Can't calculate size:", err)
			}
			if have != tc.size {
				t.Errorf("Expected size %d, got %d", tc.size, have)
			}
		})
	}
}

func TestCopyType(t *testing.T) {
	_ = copyType(Void{})

	in := &Int{Size: 4}
	out := copyType(in)

	in.Size = 8
	if size := out.(*Int).Size; size != 4 {
		t.Error("Copy doesn't make a copy, expected size 4, got", size)
	}

	t.Run("cyclical", func(t *testing.T) {
		ptr := &Pointer{}
		foo := &Struct{
			Members: []Member{
				{Type: ptr},
			},
		}
		ptr.Target = foo

		_ = copyType(foo)
	})
}

// The following are valid Types.
//
// There currently is no better way to document which
// types implement an interface.
func ExampleType_validTypes() {
	var t Type
	t = &Void{}
	t = &Int{}
	t = &Pointer{}
	t = &Array{}
	t = &Struct{}
	t = &Union{}
	t = &Enum{}
	t = &Fwd{}
	t = &Typedef{}
	t = &Volatile{}
	t = &Const{}
	t = &Restrict{}
	t = &Func{}
	t = &FuncProto{}
	t = &Var{}
	t = &Datasec{}
	_ = t
}
