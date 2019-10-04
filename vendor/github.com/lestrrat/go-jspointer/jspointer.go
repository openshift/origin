package jspointer

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/lestrrat/go-structinfo"
)

var ctxPool = sync.Pool{
	New: moreCtx,
}

func moreCtx() interface{} {
	return &matchCtx{}
}

func getCtx() *matchCtx {
	return ctxPool.Get().(*matchCtx)
}

func releaseCtx(ctx *matchCtx) {
	ctx.err = nil
	ctx.set = false
	ctx.tokens = nil
	ctx.result = nil
	ctxPool.Put(ctx)
}

// New creates a new JSON pointer for given path spec. If the path fails
// to be parsed, an error is returned
func New(path string) (*JSPointer, error) {
	var p JSPointer
	dtokens, err := parse(path)
	if err != nil {
		return nil, err
	}
	p.raw = path
	p.tokens = dtokens
	return &p, nil
}

func parse(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}

	if s[0] != Separator {
		return nil, ErrInvalidPointer
	}

	prev := 0
	tokens := []string{}
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case Separator:
			tokens = append(tokens, s[prev+1:i])
			prev = i
		}
	}

	if prev != len(s) {
		tokens = append(tokens, s[prev+1:])
	}

	dtokens := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.Replace(strings.Replace(t, EncodedSlash, "/", -1), EncodedTilde, "~", -1)
		dtokens = append(dtokens, t)
	}

	return dtokens, nil
}

// String returns the stringified version of this JSON pointer
func (p JSPointer) String() string {
	return p.raw
}

// Get applies the JSON pointer to the given item, and returns
// the result.
func (p JSPointer) Get(item interface{}) (interface{}, error) {
	ctx := getCtx()
	defer releaseCtx(ctx)

	ctx.raw = p.raw
	ctx.tokens = p.tokens
	ctx.apply(item)
	return ctx.result, ctx.err
}

// Set applies the JSON pointer to the given item, and sets the
// value accordingly.
func (p JSPointer) Set(item interface{}, value interface{}) error {
	ctx := getCtx()
	defer releaseCtx(ctx)

	ctx.set = true
	ctx.raw = p.raw
	ctx.tokens = p.tokens
	ctx.setvalue = value
	ctx.apply(item)
	return ctx.err
}

type matchCtx struct {
	err      error
	raw      string
	result   interface{}
	set      bool
	setvalue interface{}
	tokens   []string
}

func (e ErrNotFound) Error() string {
	return "match to JSON pointer not found: " + e.Ptr
}

var strType = reflect.TypeOf("")

func (c *matchCtx) apply(item interface{}) {
	if len(c.tokens) == 0 {
		c.result = item
		return
	}

	lastidx := len(c.tokens) - 1
	node := item
	for tidx, token := range c.tokens {
		v := reflect.ValueOf(node)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		switch v.Kind() {
		case reflect.Struct:
			fn := structinfo.StructFieldFromJSONName(v, token)
			if fn == "" {
				c.err = ErrNotFound{Ptr: c.raw}
				return
			}
			f := v.FieldByName(fn)
			if tidx == lastidx {
				if c.set {
					if !f.CanSet() {
						c.err = ErrCanNotSet
						return
					}
					f.Set(reflect.ValueOf(c.setvalue))
					return
				}
				c.result = f.Interface()
				return
			}
			node = f.Interface()
		case reflect.Map:
			var vt reflect.Value
			// We shall try to inflate the token to its Go native
			// type if it's not a string. In other words, try not to
			// outdo yourselves.
			if t := v.Type().Key(); t != strType {
				vt = reflect.New(t).Elem()
				if err := json.Unmarshal([]byte(token), vt.Addr().Interface()); err != nil {
					name := t.PkgPath() + "." + t.Name()
					if name == "" {
						name = "(anonymous type)"
					}
					c.err = errors.New("unsupported conversion of string to " + name)
					return
				}
			} else {
				vt = reflect.ValueOf(token)
			}
			n := v.MapIndex(vt)
			if (reflect.Value{}) == n {
				c.err = ErrNotFound{Ptr: c.raw}
				return
			}

			if tidx == lastidx {
				if c.set {
					v.SetMapIndex(vt, reflect.ValueOf(c.setvalue))
				} else {
					c.result = n.Interface()
				}
				return
			}

			node = n.Interface()
		case reflect.Slice:
			m := node.([]interface{})
			wantidx, err := strconv.Atoi(token)
			if err != nil {
				c.err = err
				return
			}

			if wantidx < 0 || len(m) <= wantidx {
				c.err = ErrSliceIndexOutOfBounds
				return
			}

			if tidx == lastidx {
				if c.set {
					m[wantidx] = c.setvalue
				} else {
					c.result = m[wantidx]
				}
				return
			}
			node = m[wantidx]
		default:
			c.err = ErrNotFound{Ptr: c.raw}
			return
		}
	}

	// If you fell through here, there was a big problem
	c.err = ErrNotFound{Ptr: c.raw}
}
