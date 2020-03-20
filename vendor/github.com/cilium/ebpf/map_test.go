package ebpf

import (
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"unsafe"

	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/unix"

	"github.com/pkg/errors"
)

func TestMain(m *testing.M) {
	err := unix.Setrlimit(8, &unix.Rlimit{
		Cur: math.MaxUint64,
		Max: math.MaxUint64,
	})
	if err != nil {
		fmt.Println("WARNING: Failed to adjust rlimit, tests may fail")
	}
	os.Exit(m.Run())
}

func TestMap(t *testing.T) {
	m := createArray(t)
	defer m.Close()

	t.Log(m)

	if err := m.Put(uint32(0), uint32(42)); err != nil {
		t.Fatal("Can't put:", err)
	}
	if err := m.Put(uint32(1), uint32(4242)); err != nil {
		t.Fatal("Can't put:", err)
	}

	m2, err := m.Clone()
	if err != nil {
		t.Fatal("Can't clone map:", err)
	}
	defer m2.Close()

	m.Close()
	m = m2

	var v uint32
	if err := m.Lookup(uint32(0), &v); err != nil {
		t.Fatal("Can't lookup 0:", err)
	}
	if v != 42 {
		t.Error("Want value 42, got", v)
	}

	var k uint32
	if err := m.NextKey(uint32(0), &k); err != nil {
		t.Fatal("Can't get:", err)
	}
	if k != 1 {
		t.Error("Want key 1, got", k)
	}
}

func TestMapClose(t *testing.T) {
	m := createArray(t)

	if err := m.Close(); err != nil {
		t.Fatal("Can't close map:", err)
	}

	if err := m.Put(uint32(0), uint32(42)); errors.Cause(err) != errClosedFd {
		t.Fatal("Put doesn't check for closed fd", err)
	}

	if _, err := m.LookupBytes(uint32(0)); errors.Cause(err) != errClosedFd {
		t.Fatal("Get doesn't check for closed fd", err)
	}
}

func TestMapCloneNil(t *testing.T) {
	m, err := (*Map)(nil).Clone()
	if err != nil {
		t.Fatal(err)
	}

	if m != nil {
		t.Fatal("Cloning a nil map doesn't return nil")
	}
}

func TestMapPin(t *testing.T) {
	m := createArray(t)
	defer m.Close()

	if err := m.Put(uint32(0), uint32(42)); err != nil {
		t.Fatal("Can't put:", err)
	}

	tmp, err := ioutil.TempDir("/sys/fs/bpf", "ebpf-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	path := filepath.Join(tmp, "map")
	if err := m.Pin(path); err != nil {
		t.Fatal(err)
	}
	m.Close()

	m, err = LoadPinnedMap(path)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	var v uint32
	if err := m.Lookup(uint32(0), &v); err != nil {
		t.Fatal("Can't lookup 0:", err)
	}
	if v != 42 {
		t.Error("Want value 42, got", v)
	}
}

func createArray(t *testing.T) *Map {
	t.Helper()

	m, err := NewMap(&MapSpec{
		Type:       Array,
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestMapInMap(t *testing.T) {
	for _, typ := range []MapType{ArrayOfMaps, HashOfMaps} {
		t.Run(typ.String(), func(t *testing.T) {
			spec := &MapSpec{
				Type:       typ,
				KeySize:    4,
				MaxEntries: 2,
				InnerMap: &MapSpec{
					Type:       Array,
					KeySize:    4,
					ValueSize:  4,
					MaxEntries: 2,
				},
			}

			inner, err := NewMap(spec.InnerMap)
			if err != nil {
				t.Fatal(err)
			}
			if err := inner.Put(uint32(1), uint32(4242)); err != nil {
				t.Fatal(err)
			}
			defer inner.Close()

			outer, err := NewMap(spec)
			if err != nil {
				t.Fatal(err)
			}
			defer outer.Close()

			if err := outer.Put(uint32(0), inner); err != nil {
				t.Fatal("Can't put inner map:", err)
			}

			var inner2 *Map
			if err := outer.Lookup(uint32(0), &inner2); err != nil {
				t.Fatal("Can't lookup 0:", err)
			}
			defer inner2.Close()

			var v uint32
			if err := inner2.Lookup(uint32(1), &v); err != nil {
				t.Fatal("Can't lookup 1 in inner2:", err)
			}

			if v != 4242 {
				t.Error("Expected value 4242, got", v)
			}

			inner2.Close()

			// Make sure we can still access the original map
			if err := inner.Lookup(uint32(1), &v); err != nil {
				t.Fatal("Can't lookup 1 in inner:", err)
			}

			if v != 4242 {
				t.Error("Expected value 4242, got", v)
			}
		})
	}
}

func TestPerfEventArray(t *testing.T) {
	m, err := NewMap(&MapSpec{
		Type: PerfEventArray,
	})
	if err != nil {
		t.Fatal("Can't create perf event array:", err)
	}
	m.Close()
}

func TestMapInMapABI(t *testing.T) {
	m := createMapInMap(t, ArrayOfMaps)
	defer m.Close()

	if m.abi.InnerMap == nil {
		t.Error("ABI is missing InnerMap")
	}
}

func createMapInMap(t *testing.T, typ MapType) *Map {
	t.Helper()

	spec := &MapSpec{
		Type:       typ,
		KeySize:    4,
		MaxEntries: 2,
		InnerMap: &MapSpec{
			Type:       Array,
			KeySize:    4,
			ValueSize:  4,
			MaxEntries: 2,
		},
	}

	m, err := NewMap(spec)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestMapInMapValueSize(t *testing.T) {
	spec := &MapSpec{
		Type:       ArrayOfMaps,
		KeySize:    4,
		ValueSize:  0,
		MaxEntries: 2,
		InnerMap: &MapSpec{
			Type:       Array,
			KeySize:    4,
			ValueSize:  4,
			MaxEntries: 2,
		},
	}

	if _, err := NewMap(spec); err != nil {
		t.Fatal(err)
	}

	spec.ValueSize = 4
	if _, err := NewMap(spec); err != nil {
		t.Fatal(err)
	}

	spec.ValueSize = 1
	if _, err := NewMap(spec); err == nil {
		t.Fatal("Expected an error")
	}
}

func TestIterateEmptyMap(t *testing.T) {
	hash := createHash()
	defer hash.Close()

	entries := hash.Iterate()

	var key string
	var value uint32
	if entries.Next(&key, &value) != false {
		t.Error("Empty map should not be iterable")
	}
}

func TestMapIterate(t *testing.T) {
	hash, err := NewMap(&MapSpec{
		Type:       Hash,
		KeySize:    5,
		ValueSize:  4,
		MaxEntries: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer hash.Close()

	if err := hash.Put("hello", uint32(21)); err != nil {
		t.Fatal(err)
	}

	if err := hash.Put("world", uint32(42)); err != nil {
		t.Fatal(err)
	}

	var key string
	var value uint32
	var keys []string

	entries := hash.Iterate()
	for entries.Next(&key, &value) {
		keys = append(keys, key)
	}

	if err := entries.Err(); err != nil {
		t.Fatal(err)
	}

	sort.Strings(keys)

	if n := len(keys); n != 2 {
		t.Fatal("Expected to get 2 keys, have", n)
	}
	if keys[0] != "hello" {
		t.Error("Expected index 0 to be hello, got", keys[0])
	}
	if keys[1] != "world" {
		t.Error("Expected index 1 to be hello, got", keys[1])
	}
}

func TestNotExist(t *testing.T) {
	hash := createHash()
	defer hash.Close()

	var tmp uint32
	err := hash.Lookup("hello", &tmp)
	if !IsNotExist(err) {
		t.Error("IsNotExist returns false for missing key")
	}

	buf, err := hash.LookupBytes("hello")
	if err != nil {
		t.Error("Looking up non-existent key return an error:", err)
	}
	if buf != nil {
		t.Error("LookupBytes returns non-nil buffer for non-existent key")
	}

	if err := hash.Delete("hello"); !IsNotExist(err) {
		t.Error("Deleting unknown key doesn't return a non-existing error")
	}

	if err := hash.NextKey(nil, &tmp); !IsNotExist(err) {
		t.Error("Looking up next key in empty map doesn't return a non-existing error")
	}
}

func TestIterateMapInMap(t *testing.T) {
	const idx = uint32(1)

	parent := createMapInMap(t, ArrayOfMaps)
	defer parent.Close()

	a := createArray(t)
	defer a.Close()

	if err := parent.Put(idx, a); err != nil {
		t.Fatal(err)
	}

	var (
		key     uint32
		m       *Map
		entries = parent.Iterate()
	)
	defer m.Close()

	if !entries.Next(&key, &m) {
		t.Fatal("Iterator encountered error:", entries.Err())
	}

	if key != 1 {
		t.Error("Iterator didn't skip first entry")
	}

	if m == nil {
		t.Fatal("Map is nil")
	}
}

func TestPerCPUMarshaling(t *testing.T) {
	numCPU, err := internal.PossibleCPUs()
	if err != nil {
		t.Fatal(err)
	}
	if numCPU < 2 {
		t.Skip("Test requires at least two CPUs")
	}

	arr, err := NewMap(&MapSpec{
		Type:       PerCPUArray,
		KeySize:    4,
		ValueSize:  5,
		MaxEntries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer arr.Close()

	values := []*customEncoding{
		&customEncoding{"hello"},
		&customEncoding{"world"},
	}
	if err := arr.Put(uint32(0), values); err != nil {
		t.Fatal(err)
	}

	// Make sure unmarshaling works on slices containing pointers
	var retrieved []*customEncoding
	if err := arr.Lookup(uint32(0), &retrieved); err != nil {
		t.Fatal("Can't retrieve key 0:", err)
	}

	for i, want := range []string{"HELLO", "WORLD"} {
		if retrieved[i] == nil {
			t.Error("First item is nil")
		} else if have := retrieved[i].data; have != want {
			t.Errorf("Put doesn't use BinaryMarshaler, expected %s but got %s", want, have)
		}
	}
}

func TestMapMarshalUnsafe(t *testing.T) {
	m, err := NewMap(&MapSpec{
		Type:       Hash,
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	key := uint32(1)
	value := uint32(42)

	if err := m.Put(unsafe.Pointer(&key), unsafe.Pointer(&value)); err != nil {
		t.Fatal(err)
	}

	var res uint32
	if err := m.Lookup(unsafe.Pointer(&key), unsafe.Pointer(&res)); err != nil {
		t.Fatal("Can't get item:", err)
	}

	var sum uint32
	iter := m.Iterate()
	for iter.Next(&key, unsafe.Pointer(&res)) {
		sum += res
	}
	if err := iter.Err(); err != nil {
		t.Fatal(err)
	}

	if res != 42 {
		t.Fatalf("Expected 42, got %d", res)
	}

	iter = m.Iterate()
	iter.Next(unsafe.Pointer(&key), &res)
	if err := iter.Err(); err != nil {
		t.Error(err)
	}
	if key != 1 {
		t.Errorf("Expected key 1, got %d", key)
	}

	if err := m.Delete(unsafe.Pointer(&key)); err != nil {
		t.Fatal("Can't delete:", err)
	}
}

func TestMapName(t *testing.T) {
	m, err := NewMap(&MapSpec{
		Name:       "test",
		Type:       Array,
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	info, err := bpfGetMapInfoByFD(m.fd)
	if err != nil {
		t.Fatal(err)
	}

	if name := convertCString(info.mapName[:]); name != "test" {
		t.Error("Expected name to be test, got", name)
	}
}

func TestMapFromFD(t *testing.T) {
	m := createArray(t)
	if err := m.Put(uint32(0), uint32(123)); err != nil {
		t.Fatal(err)
	}

	// If you're thinking about copying this, don't. Use
	// Clone() instead.
	m2, err := NewMapFromFD(m.FD())
	if err != nil {
		t.Fatal(err)
	}

	var val uint32
	if err := m2.Lookup(uint32(0), &val); err != nil {
		t.Fatal("Can't look up key:", err)
	}

	if val != 123 {
		t.Error("Wrong value")
	}
}

type benchValue struct {
	ID      uint32
	Val16   uint16
	Val16_2 uint16
	Name    [8]byte
	LID     uint64
}

type customBenchValue benchValue

func (cbv *customBenchValue) UnmarshalBinary(buf []byte) error {
	cbv.ID = internal.NativeEndian.Uint32(buf)
	cbv.Val16 = internal.NativeEndian.Uint16(buf[4:])
	cbv.Val16_2 = internal.NativeEndian.Uint16(buf[6:])
	copy(cbv.Name[:], buf[8:])
	cbv.LID = internal.NativeEndian.Uint64(buf[16:])
	return nil
}

func (cbv *customBenchValue) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 24)
	internal.NativeEndian.PutUint32(buf, cbv.ID)
	internal.NativeEndian.PutUint16(buf[4:], cbv.Val16)
	internal.NativeEndian.PutUint16(buf[6:], cbv.Val16_2)
	copy(buf[8:], cbv.Name[:])
	internal.NativeEndian.PutUint64(buf[16:], cbv.LID)
	return buf, nil
}

func BenchmarkMarshalling(b *testing.B) {
	newMap := func(valueSize uint32) *Map {
		m, err := NewMap(&MapSpec{
			Type:       Hash,
			KeySize:    8,
			ValueSize:  valueSize,
			MaxEntries: 1,
		})
		if err != nil {
			b.Fatal(err)
		}
		return m
	}

	key := uint64(0)

	m := newMap(24)
	if err := m.Put(key, benchValue{}); err != nil {
		b.Fatal(err)
	}

	b.Run("reflection", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		var value benchValue

		for i := 0; i < b.N; i++ {
			err := m.Lookup(unsafe.Pointer(&key), &value)
			if err != nil {
				b.Fatal("Can't get key:", err)
			}
		}
	})

	b.Run("custom", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		var value customBenchValue

		for i := 0; i < b.N; i++ {
			err := m.Lookup(unsafe.Pointer(&key), &value)
			if err != nil {
				b.Fatal("Can't get key:", err)
			}
		}
	})

	b.Run("unsafe", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		var value benchValue

		for i := 0; i < b.N; i++ {
			err := m.Lookup(unsafe.Pointer(&key), unsafe.Pointer(&value))
			if err != nil {
				b.Fatal("Can't get key:", err)
			}
		}
	})
}

func BenchmarkMap(b *testing.B) {
	m, err := NewMap(&MapSpec{
		Type:       Hash,
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 1,
	})
	if err != nil {
		b.Fatal(err)
	}

	if err := m.Put(uint32(0), uint32(42)); err != nil {
		b.Fatal(err)
	}

	b.Run("Lookup", func(b *testing.B) {
		var key, value uint32

		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			err := m.Lookup(unsafe.Pointer(&key), unsafe.Pointer(&value))
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Update", func(b *testing.B) {
		var key, value uint32

		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			err := m.Update(unsafe.Pointer(&key), unsafe.Pointer(&value), UpdateAny)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("NextKey", func(b *testing.B) {
		var key uint32

		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			err := m.NextKey(nil, unsafe.Pointer(&key))
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Delete", func(b *testing.B) {
		var key uint32

		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			err := m.Delete(unsafe.Pointer(&key))
			if err != nil && !IsNotExist(err) {
				b.Fatal(err)
			}
		}
	})
}

// Per CPU maps store a distinct value for each CPU. They are useful
// to collect metrics.
func ExampleMap_perCPU() {
	arr, err := NewMap(&MapSpec{
		Type:       PerCPUArray,
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 2,
	})
	if err != nil {
		panic(err)
	}

	first := []uint32{4, 5}
	if err := arr.Put(uint32(0), first); err != nil {
		panic(err)
	}

	second := []uint32{2, 8}
	if err := arr.Put(uint32(1), second); err != nil {
		panic(err)
	}

	var values []uint32
	if err := arr.Lookup(uint32(0), &values); err != nil {
		panic(err)
	}
	fmt.Println("First two values:", values[:2])

	var (
		key     uint32
		entries = arr.Iterate()
	)

	for entries.Next(&key, &values) {
		// NB: sum can overflow, real code should check for this
		var sum uint32
		for _, n := range values {
			sum += n
		}
		fmt.Printf("Sum of %d: %d\n", key, sum)
	}

	if err := entries.Err(); err != nil {
		panic(err)
	}
}

// It is possible to use unsafe.Pointer to avoid marshalling
// and copy overhead. It is the resposibility of the caller to ensure
// the correct size of unsafe.Pointers.
//
// Note that using unsafe.Pointer is only marginally faster than
// implementing Marshaler on the type.
func ExampleMap_zeroCopy() {
	hash := createHash()
	defer hash.Close()

	key := [5]byte{'h', 'e', 'l', 'l', 'o'}
	value := uint32(23)

	if err := hash.Put(unsafe.Pointer(&key), unsafe.Pointer(&value)); err != nil {
		panic(err)
	}

	value = 0
	if err := hash.Lookup(unsafe.Pointer(&key), unsafe.Pointer(&value)); err != nil {
		panic("can't get value:" + err.Error())
	}

	fmt.Printf("The value is: %d\n", value)
	// Output: The value is: 23
}

func createHash() *Map {
	hash, err := NewMap(&MapSpec{
		Type:       Hash,
		KeySize:    5,
		ValueSize:  4,
		MaxEntries: 10,
	})
	if err != nil {
		panic(err)
	}
	return hash
}

func ExampleMap_NextKey() {
	hash := createHash()
	defer hash.Close()

	if err := hash.Put("hello", uint32(21)); err != nil {
		panic(err)
	}

	if err := hash.Put("world", uint32(42)); err != nil {
		panic(err)
	}

	var firstKey string
	if err := hash.NextKey(nil, &firstKey); err != nil {
		panic(err)
	}

	var nextKey string
	if err := hash.NextKey(firstKey, &nextKey); err != nil {
		panic(err)
	}

	// Order of keys is non-deterministic due to randomized map seed
}

// ExampleMap_Iterate demonstrates how to iterate over all entries
// in a map.
func ExampleMap_Iterate() {
	hash := createHash()
	defer hash.Close()

	if err := hash.Put("hello", uint32(21)); err != nil {
		panic(err)
	}

	if err := hash.Put("world", uint32(42)); err != nil {
		panic(err)
	}

	var (
		key     string
		value   uint32
		entries = hash.Iterate()
	)

	for entries.Next(&key, &value) {
		// Order of keys is non-deterministic due to randomized map seed
		fmt.Printf("key: %s, value: %d\n", key, value)
	}

	if err := entries.Err(); err != nil {
		panic(fmt.Sprint("Iterator encountered an error:", err))
	}
}

// It is possible to iterate nested maps and program arrays by
// unmarshaling into a *Map or *Program.
func ExampleMap_Iterate_nestedMapsAndProgramArrays() {
	var arrayOfMaps *Map // Set this up somehow

	var (
		key     uint32
		m       *Map
		entries = arrayOfMaps.Iterate()
	)

	// Make sure that the iterated map is closed after
	// we are done.
	defer m.Close()

	for entries.Next(&key, &m) {
		// Order of keys is non-deterministic due to randomized map seed
		fmt.Printf("key: %v, map: %v\n", key, m)
	}

	if err := entries.Err(); err != nil {
		panic(fmt.Sprint("Iterator encountered an error:", err))
	}
}
