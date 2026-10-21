package goconst

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// treeVisitor is used to walk the AST and find strings that could be constants.
type treeVisitor struct {
	fileSet     *token.FileSet
	typeInfo    *types.Info
	packageName string
	p           *Parser
	ignoreRegex *regexp.Regexp

	// skipNodes holds map-key expression subtrees to skip entirely while
	// walking, used by -ignore-map-keys for keys that are not plain literals
	// (e.g. NamedString("key")). Populated per file; the walk is single
	// threaded so no synchronization is needed.
	skipNodes map[ast.Node]struct{}
}

// Visit browses the AST tree for strings that could be potentially
// replaced by constants.
// A map of existing constants is built as well (-match-constant).
func (v *treeVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return v
	}

	// Prune map-key expression subtrees flagged by -ignore-map-keys so their
	// string literals are never recorded (e.g. the "key" in K("key")).
	if _, skip := v.skipNodes[node]; skip {
		return nil
	}

	// A single case with "ast.BasicLit" would be much easier
	// but then we wouldn't be able to tell in which context
	// the string is defined (could be a constant definition).
	switch t := node.(type) {
	// Scan for constants in an attempt to match strings with existing constants
	case *ast.GenDecl:
		if !v.p.matchConstant && !v.p.findDuplicates {
			return v
		}
		if t.Tok != token.CONST {
			return v
		}

		for _, spec := range t.Specs {
			val := spec.(*ast.ValueSpec)
			if v.typeInfo != nil && v.p.evalConstExpressions {
				if len(v.typeInfo.Defs) > 0 {
					addedFromDefs := false
					for _, name := range val.Names {
						obj, ok := v.typeInfo.Defs[name].(*types.Const)
						if !ok || obj.Val() == nil || !v.isSupportedKind(obj.Val().Kind()) {
							continue
						}

						displayValue, valueKey := constValueStrings(obj.Val())
						v.addConst(name.Name, displayValue, name.Pos(), valueKey)
						addedFromDefs = true
					}
					if addedFromDefs || len(val.Values) == 0 {
						continue
					}
				}
			}

			for i, str := range val.Values {
				if v.typeInfo != nil && v.p.evalConstExpressions {
					typedVal, ok := v.typeInfo.Types[str]
					if !ok || typedVal.Value == nil || !v.isSupportedKind(typedVal.Value.Kind()) {
						continue
					}

					displayValue, valueKey := constValueStrings(typedVal.Value)
					v.addConst(val.Names[i].Name, displayValue, str.Pos(), valueKey)
					continue
				}

				lit, ok := str.(*ast.BasicLit)
				if !ok || !v.isSupported(lit.Kind) {
					continue
				}
				v.addConst(val.Names[i].Name, lit.Value, val.Names[i].Pos())
			}
		}

	// foo := "moo"
	case *ast.AssignStmt:
		for _, rhs := range t.Rhs {
			lit, ok := rhs.(*ast.BasicLit)
			if !ok || !v.isSupported(lit.Kind) {
				continue
			}

			v.addString(lit.Value, rhs.(*ast.BasicLit).Pos(), Assignment)
		}

	// if foo == "moo"
	case *ast.BinaryExpr:
		if t.Op != token.EQL && t.Op != token.NEQ {
			return v
		}

		var lit *ast.BasicLit
		var ok bool

		lit, ok = t.X.(*ast.BasicLit)
		if ok && v.isSupported(lit.Kind) {
			v.addString(lit.Value, lit.Pos(), Binary)
		}

		lit, ok = t.Y.(*ast.BasicLit)
		if ok && v.isSupported(lit.Kind) {
			v.addString(lit.Value, lit.Pos(), Binary)
		}

	// case "foo":
	case *ast.CaseClause:
		for _, item := range t.List {
			lit, ok := item.(*ast.BasicLit)
			if ok && v.isSupported(lit.Kind) {
				v.addString(lit.Value, lit.Pos(), Case)
			}
		}

	// return "boo"
	case *ast.ReturnStmt:
		for _, item := range t.Results {
			lit, ok := item.(*ast.BasicLit)
			if ok && v.isSupported(lit.Kind) {
				v.addString(lit.Value, lit.Pos(), Return)
			}
		}

	// fn("http://")
	case *ast.CallExpr:
		if !v.shouldIgnoreCall(t) {
			for _, item := range t.Args {
				lit, ok := item.(*ast.BasicLit)
				if ok && v.isSupported(lit.Kind) {
					v.addString(lit.Value, lit.Pos(), Call)
				}
			}
		}

	// []string{"foo"}, map[string]string{"k": "v"}, struct{A string}{A: "foo"}
	case *ast.CompositeLit:
		// isMap is only consulted for pruning expression keys, which matters
		// only when -ignore-map-keys is set.
		isMap := false
		if v.p.ignoreMapKeys {
			isMap = v.isMapLiteral(t)
		}
		for _, item := range t.Elts {
			v.addCompositeLiteralElement(item, isMap)
		}
	}

	return v
}

func constValueStrings(val constant.Value) (string, string) {
	if val.Kind() == constant.String {
		return val.ExactString(), "string:" + constant.StringVal(val)
	}
	return val.String(), val.Kind().String() + ":" + val.ExactString()
}

// isMapLiteral reports whether a composite literal is a map. It prefers type
// information (which resolves named map types and elided nested literals) and
// falls back to the syntactic type when none is available (e.g. the CLI path,
// where only explicit map[...]... literals can be recognized).
func (v *treeVisitor) isMapLiteral(t *ast.CompositeLit) bool {
	if v.typeInfo != nil {
		if tv, ok := v.typeInfo.Types[t]; ok && tv.Type != nil {
			_, isMap := tv.Type.Underlying().(*types.Map)
			return isMap
		}
	}
	_, ok := t.Type.(*ast.MapType)
	return ok
}

func (v *treeVisitor) addCompositeLiteralElement(node ast.Expr, isMap bool) {
	if lit, ok := node.(*ast.BasicLit); ok && v.isSupported(lit.Kind) {
		v.addString(lit.Value, lit.Pos(), CompositeLit)
		return
	}

	kv, ok := node.(*ast.KeyValueExpr)
	if !ok {
		return
	}

	if keyLit, ok := kv.Key.(*ast.BasicLit); ok {
		// Direct literal key. A string literal can only be a map key (struct
		// keys are identifiers, array indices are integers), so it is dropped
		// when requested with no type information; numeric keys are out of
		// scope and kept.
		if v.isSupported(keyLit.Kind) && (!v.p.ignoreMapKeys || keyLit.Kind != token.STRING) {
			v.addString(keyLit.Value, keyLit.Pos(), CompositeLit)
		}
	} else if v.p.ignoreMapKeys && isMap {
		// Key is an expression (e.g. NamedString("key")); any string literal it
		// contains is part of the map key, so skip the whole subtree. Gated on
		// isMap so array indices such as [10]int{int(2): 1} are left alone.
		v.markSkip(kv.Key)
	}

	if valueLit, ok := kv.Value.(*ast.BasicLit); ok && v.isSupported(valueLit.Kind) {
		v.addString(valueLit.Value, valueLit.Pos(), CompositeLit)
	}
}

// markSkip flags an AST node so the walk prunes it and its subtree.
func (v *treeVisitor) markSkip(node ast.Node) {
	if v.skipNodes == nil {
		v.skipNodes = make(map[ast.Node]struct{})
	}
	v.skipNodes[node] = struct{}{}
}

// shouldIgnoreCall returns true if the call expression matches a function
// name in the ignoreFunctions set. Supports direct calls (e.g., "println")
// and one-level qualified calls (e.g., "slog.Info").
func (v *treeVisitor) shouldIgnoreCall(call *ast.CallExpr) bool {
	if len(v.p.ignoreFunctions) == 0 {
		return false
	}
	var name string
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		name = fn.Name
	case *ast.SelectorExpr:
		if ident, ok := fn.X.(*ast.Ident); ok {
			name = ident.Name + "." + fn.Sel.Name
		}
	}
	if name == "" {
		return false
	}
	_, found := v.p.ignoreFunctions[name]
	return found
}

// addString adds a string in the map along with its position in the tree.
func (v *treeVisitor) addString(str string, pos token.Pos, typ Type) {
	// Early type exclusion check
	ok, excluded := v.p.excludeTypes[typ]
	if ok && excluded {
		return
	}

	// Drop quotes if any
	var unquotedStr string
	if strings.HasPrefix(str, `"`) || strings.HasPrefix(str, "`") {
		var err error
		unquotedStr, err = strconv.Unquote(str)
		if err != nil {
			// Reuse strings from pool if possible to avoid allocations
			sb := GetStringBuilder()
			defer PutStringBuilder(sb)

			// If unquoting fails, manually strip quotes
			// This avoids additional temporary strings
			if len(str) >= 2 {
				sb.WriteString(str[1 : len(str)-1])
				unquotedStr = sb.String()
			} else {
				unquotedStr = str
			}
		}
	} else {
		unquotedStr = str
	}

	// Early length check
	if len(unquotedStr) == 0 || utf8.RuneCountInString(unquotedStr) < v.p.minLength {
		return
	}

	// Early regex filtering - pre-compiled for efficiency
	if v.ignoreRegex != nil && v.ignoreRegex.MatchString(unquotedStr) {
		return
	}

	// Early number range filtering
	if v.p.numberMin != 0 || v.p.numberMax != 0 {
		if i, err := strconv.ParseInt(unquotedStr, 0, 0); err == nil {
			if (v.p.numberMin != 0 && i < int64(v.p.numberMin)) ||
				(v.p.numberMax != 0 && i > int64(v.p.numberMax)) {
				return
			}
		}
	}

	// Use interned string to reduce memory usage - identical strings share the same memory
	internedStr := InternString(unquotedStr)

	// Update the count for fast threshold checks in ProcessResults
	v.p.IncrementStringCount(internedStr)

	// Record every occurrence so that position lists and display counts stay accurate
	v.p.stringMutex.Lock()
	defer v.p.stringMutex.Unlock()

	if _, exists := v.p.strs[internedStr]; !exists {
		v.p.strs[internedStr] = make([]ExtendedPos, 0, v.p.minOccurrences)
	}

	v.p.strs[internedStr] = append(v.p.strs[internedStr], ExtendedPos{
		packageName: InternString(v.packageName),
		Position:    v.fileSet.Position(pos),
	})
}

// addConst adds a const in the map along with its position in the tree.
func (v *treeVisitor) addConst(name string, val string, pos token.Pos, valueKey ...string) {
	// Early filtering using the same criteria as for strings
	var unquotedVal string
	if strings.HasPrefix(val, `"`) || strings.HasPrefix(val, "`") {
		var err error
		// Use string builder from pool to reduce allocations
		sb := GetStringBuilder()
		defer PutStringBuilder(sb)

		if unquotedVal, err = strconv.Unquote(val); err != nil {
			// If unquoting fails, manually strip quotes without allocations
			if len(val) >= 2 {
				sb.WriteString(val[1 : len(val)-1])
				unquotedVal = sb.String()
			} else {
				unquotedVal = val
			}
		}
	} else {
		unquotedVal = val
	}

	// Skip constants with values that would be filtered anyway
	if utf8.RuneCountInString(unquotedVal) < v.p.minLength {
		return
	}

	if v.ignoreRegex != nil && v.ignoreRegex.MatchString(unquotedVal) {
		return
	}

	// Use interned string to reduce memory usage
	internedVal := InternString(unquotedVal)
	internedName := InternString(name)
	internedPkg := InternString(v.packageName)
	internedKey := internedVal
	if len(valueKey) > 0 && valueKey[0] != "" {
		internedKey = InternString(valueKey[0])
	}

	// Lock to safely update the shared map
	v.p.constMutex.Lock()
	defer v.p.constMutex.Unlock()

	// Collect the constant when it is the first with this value, when
	// duplicate detection needs all of them, or when constant matching
	// needs all of them to pick the best per scope.
	if _, ok := v.p.consts[internedVal]; !ok || v.p.findDuplicates || v.p.matchConstant {
		v.p.consts[internedVal] = append(v.p.consts[internedVal], ConstType{
			Name:        internedName,
			packageName: internedPkg,
			valueKey:    internedKey,
			Position:    v.fileSet.Position(pos),
		})
	}
}

func (v *treeVisitor) isSupported(tk token.Token) bool {
	for _, s := range v.p.supportedTokens {
		if tk == s {
			return true
		}
	}
	return false
}

func (v *treeVisitor) isSupportedKind(kind constant.Kind) bool {
	for _, s := range v.p.supportedKinds {
		if kind == s {
			return true
		}
	}
	return false
}
