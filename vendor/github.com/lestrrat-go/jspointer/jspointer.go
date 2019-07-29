package jspointer

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"

	"github.com/lestrrat-go/structinfo"
)

type tokens struct {
	s         string
	positions [][2]int
}

func (t *tokens) size() int {
	return len(t.positions)
}

func (t *tokens) get(i int) string {
	p := t.positions[i]
	return t.s[p[0]:p[1]]
}

// New creates a new JSON pointer for given path spec. If the path fails
// to be parsed, an error is returned
func New(path string) (*JSPointer, error) {
	var p JSPointer

	if err := p.parse(path); err != nil {
		return nil, err
	}
	p.raw = path
	return &p, nil
}

func (p *JSPointer) parse(s string) error {
	if s == "" {
		return nil
	}

	if s[0] != Separator {
		return ErrInvalidPointer
	}

	if len(s) < 2 {
		return ErrInvalidPointer
	}

	ntokens := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			ntokens++
		}
	}

	positions := make([][2]int, 0, ntokens)
	start := 1
	var buf bytes.Buffer
	buf.WriteByte(s[0])
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case Separator:
			buf.WriteByte(s[i])
			positions = append(positions, [2]int{start, buf.Len() - 1})
			start = i + 1
		case '~':
			if len(s) == 1 {
				buf.WriteByte(s[i])
			} else {
				switch s[1] {
				case '0':
					buf.WriteByte('~')
				case '1':
					buf.WriteByte('/')
				default:
					buf.WriteByte(s[i])
				}
			}
		default:
			buf.WriteByte(s[i])
		}
	}

	if start < buf.Len() {
		positions = append(positions, [2]int{start, buf.Len()})
	}

	p.tokens.s = buf.String()
	p.tokens.positions = positions
	return nil
}

// String returns the stringified version of this JSON pointer
func (p JSPointer) String() string {
	return p.raw
}

// Get applies the JSON pointer to the given item, and returns
// the result.
func (p JSPointer) Get(item interface{}) (interface{}, error) {
	var ctx matchCtx

	ctx.raw = p.raw
	ctx.tokens = &p.tokens
	ctx.apply(item)
	return ctx.result, ctx.err
}

// Set applies the JSON pointer to the given item, and sets the
// value accordingly.
func (p JSPointer) Set(item interface{}, value interface{}) error {
	var ctx matchCtx

	ctx.set = true
	ctx.raw = p.raw
	ctx.tokens = &p.tokens
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
	tokens   *tokens
}

func (e ErrNotFound) Error() string {
	return "match to JSON pointer not found: " + e.Ptr
}

type JSONGetter interface {
	JSONGet(tok string) (interface{}, error)
}

var strType = reflect.TypeOf("")
var zeroval reflect.Value

func (c *matchCtx) apply(item interface{}) {
	if c.tokens.size() == 0 {
		c.result = item
		return
	}

	node := item
	lastidx := c.tokens.size() - 1
	for i := 0; i < c.tokens.size(); i++ {
		token := c.tokens.get(i)

		if getter, ok := node.(JSONGetter); ok {
			x, err := getter.JSONGet(token)
			if err != nil {
				c.err = ErrNotFound{Ptr: c.raw}
				return
			}
			if i == lastidx {
				c.result = x
				return
			}
			node = x
			continue
		}
		v := reflect.ValueOf(node)

		// Does this thing implement a JSONGet?

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
			if i == lastidx {
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
			if zeroval == n {
				c.err = ErrNotFound{Ptr: c.raw}
				return
			}

			if i == lastidx {
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

			if i == lastidx {
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
