//Copyright 2013 Vastech SA (PTY) LTD
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

package gographviz

import (
	"fmt"
	"github.com/awalterschulze/gographviz/scanner"
	"github.com/awalterschulze/gographviz/token"
	"strings"
	"text/template"
	"unicode"
)

type Escape struct {
	Interface
}

//Returns a graph which will try to escape some strings when required
func NewEscape() Interface {
	return &Escape{NewGraph()}
}

func isHtml(s string) bool {
	if len(s) == 0 {
		return false
	}
	ss := strings.TrimSpace(s)
	if ss[0] != '<' {
		return false
	}
	count := 0
	for _, c := range ss {
		if c == '<' {
			count += 1
		}
		if c == '>' {
			count -= 1
		}
	}
	if count == 0 {
		return true
	}
	return false
}

func isLetter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_' ||
		ch >= 0x80 && unicode.IsLetter(ch) && ch != 'ε'
}

func isId(s string) bool {
	i := 0
	pos := false
	for _, c := range s {
		if i == 0 {
			if !isLetter(c) {
				return false
			}
			pos = true
		}
		if unicode.IsSpace(c) {
			return false
		}
		if c == '-' {
			return false
		}
		i++
	}
	return pos
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9' || ch >= 0x80 && unicode.IsDigit(ch)
}

func isNumber(s string) bool {
	state := 0
	for _, c := range s {
		if state == 0 {
			if isDigit(c) || c == '.' {
				state = 2
			} else if c == '-' {
				state = 1
			} else {
				return false
			}
		} else if state == 1 {
			if isDigit(c) || c == '.' {
				state = 2
			}
		} else if c != '.' && !isDigit(c) {
			return false
		}
	}
	return (state == 2)
}

func isStringLit(s string) bool {
	lex := &scanner.Scanner{}
	lex.Init([]byte(s), token.DOTTokens)
	tok, _ := lex.Scan()
	if tok.Type != token.DOTTokens.Type("string_lit") {
		return false
	}
	tok, _ = lex.Scan()
	if tok.Type != token.EOF {
		return false
	}
	return true
}

func esc(s string) string {
	if len(s) == 0 {
		return s
	}
	if isHtml(s) {
		return s
	}
	ss := strings.TrimSpace(s)
	if ss[0] == '<' {
		return fmt.Sprintf("\"%s\"", strings.Replace(s, "\"", "\\\"", -1))
	}
	if isId(s) {
		return s
	}
	if isNumber(s) {
		return s
	}
	if isStringLit(s) {
		return s
	}
	return fmt.Sprintf("\"%s\"", template.HTMLEscapeString(s))
}

func escAttrs(attrs map[string]string) map[string]string {
	newAttrs := make(map[string]string)
	for k, v := range attrs {
		newAttrs[esc(k)] = esc(v)
	}
	return newAttrs
}

func (this *Escape) SetName(name string) {
	this.Interface.SetName(esc(name))
}

func (this *Escape) AddPortEdge(src, srcPort, dst, dstPort string, directed bool, attrs map[string]string) {
	this.Interface.AddPortEdge(esc(src), srcPort, esc(dst), dstPort, directed, escAttrs(attrs))
}

func (this *Escape) AddEdge(src, dst string, directed bool, attrs map[string]string) {
	this.AddPortEdge(src, "", dst, "", directed, attrs)
}

func (this *Escape) AddNode(parentGraph string, name string, attrs map[string]string) {
	this.Interface.AddNode(esc(parentGraph), esc(name), escAttrs(attrs))
}

func (this *Escape) AddAttr(parentGraph string, field, value string) {
	this.Interface.AddAttr(esc(parentGraph), esc(field), esc(value))
}

func (this *Escape) AddSubGraph(parentGraph string, name string, attrs map[string]string) {
	this.Interface.AddSubGraph(esc(parentGraph), esc(name), escAttrs(attrs))
}
