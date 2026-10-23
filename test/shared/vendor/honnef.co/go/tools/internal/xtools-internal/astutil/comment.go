// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package astutil

import (
	"go/ast"
	"go/token"
	"iter"
	"sort"
	"strings"
)

// Deprecation returns the paragraph of the doc comment that starts with the
// conventional "Deprecation: " marker, as defined by
// https://go.dev/wiki/Deprecated, or "" if the documented symbol is not
// deprecated.
func Deprecation(doc *ast.CommentGroup) string {
	for p := range strings.SplitSeq(doc.Text(), "\n\n") {
		// There is still some ambiguity for deprecation message. This function
		// only returns the paragraph introduced by "Deprecated: ". More
		// information related to the deprecation may follow in additional
		// paragraphs, but the deprecation message should be able to stand on
		// its own. See golang/go#38743.
		if strings.HasPrefix(p, "Deprecated: ") {
			return p
		}
	}
	return ""
}

func Directives(g *ast.CommentGroup) (res []ast.Directive) {
	if g != nil {
		// Avoid (*ast.CommentGroup).Text() as it swallows directives.
		for _, c := range g.List {
			if d, ok := ast.ParseDirective(c.Slash, c.Text); ok {
				res = append(res, d)
			}
		}
	}
	return
}

// Comments returns an iterator over the comments overlapping the specified interval.
// Comments are sorted by position in the file, so we can use binary search.
func Comments(file *ast.File, start, end token.Pos) iter.Seq[*ast.Comment] {
	return func(yield func(*ast.Comment) bool) {
		// Find the first comment group that overlaps the range.
		i := sort.Search(len(file.Comments), func(i int) bool {
			return file.Comments[i].End() >= start
		})
		for _, cg := range file.Comments[i:] {
			if cg.Pos() > end {
				return
			}
			// Find the first comment in the group that overlaps the range.
			j := sort.Search(len(cg.List), func(j int) bool {
				return cg.List[j].End() >= start
			})
			for _, co := range cg.List[j:] {
				if co.Pos() > end {
					return
				}
				if !yield(co) {
					return
				}
			}
		}
	}
}
