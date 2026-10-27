package structure

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

const AnonymousName = "<anonymous>"

// BlankFieldName is the name go/types gives a blank struct field.
const BlankFieldName = "_"

// fieldInfo contains raw field data independent of type name.
type fieldInfo struct {
	// name is the name of the field.
	name string
	// exported indicates if the field is exported.
	exported bool //exhaustruct:optional
	// enforced indicates if the field is enforced via directive.
	enforced bool //exhaustruct:optional
	// optional indicates if the field is optional via directive.
	optional bool //exhaustruct:optional
	// embedded holds the fields an embedded field promotes, nil otherwise.
	embedded *structFields //exhaustruct:optional
}

// structFields contains field information for a struct, independent of type name.
type structFields struct {
	// strct is the type this field data was read from, and the identity the
	// promotion walk prunes by. A cached value would answer as well only while
	// one goroutine fills the cache: two passes missing at once each build a
	// value of their own, and one struct reached through two of them would then
	// read as two.
	strct *types.Struct
	// packagePath is the package path where fields are declared.
	packagePath string
	// fields is the list of fields in declaration order.
	fields []fieldInfo
}

type Struct struct {
	Name        string
	FullPath    string
	PackageName string

	Position token.Position //exhaustruct:optional
	Fields   Fields         //exhaustruct:optional

	Enforced bool //exhaustruct:optional
	Ignored  bool //exhaustruct:optional
	Optional bool //exhaustruct:optional

	PatternEnforced bool //exhaustruct:optional
	PatternIgnored  bool //exhaustruct:optional
	PatternOptional bool //exhaustruct:optional

	AllowEmptyDecl bool //exhaustruct:optional
}

// PackagePath returns the package path of the struct type.
func (s *Struct) PackagePath() string {
	if idx := strings.LastIndex(s.FullPath, "."); idx >= 0 {
		return s.FullPath[:idx]
	}

	return s.FullPath
}

// IsEnforced returns true if struct is enforced via directive or pattern.
func (s *Struct) IsEnforced() bool {
	return s.Enforced || s.PatternEnforced
}

// IsIgnored returns true if struct is ignored via directive or pattern.
func (s *Struct) IsIgnored() bool {
	return s.Ignored || s.PatternIgnored
}

// IsOptional returns true if struct is optional via directive or pattern.
func (s *Struct) IsOptional() bool {
	return s.Optional || s.PatternOptional
}

// SkippedFields returns missing required fields for a composite literal.
// callerPkgPath is used to determine if unexported fields are accessible.
// canNamePromoted tells whether the literal may write a promoted field as a key,
// which Go permits since 1.27 (golang/go#77245).
// For positional literals: returns fields after the last provided element.
// For named literals: returns fields not present in the literal.
func (s *Struct) SkippedFields(lit *ast.CompositeLit, callerPkgPath string, canNamePromoted bool) []Field {
	// An empty literal supplies nothing and can still be written in either
	// form, so it is read as a keyed literal with no keys. The two then answer
	// alike: neither can name a blank field, and both reach a field enforced
	// under an embedded one.
	if isNamedLiteral(lit) || len(lit.Elts) == 0 {
		return s.skippedNamed(lit, callerPkgPath, canNamePromoted)
	}

	return s.skippedPositional(len(lit.Elts), s.Fields.PackagePath != callerPkgPath)
}

// isNamedLiteral checks if a composite literal uses named fields. It treats
// empty literals as not named, since positional literals checks are simpler.
func isNamedLiteral(lit *ast.CompositeLit) bool {
	if len(lit.Elts) == 0 {
		return false
	}

	_, ok := lit.Elts[0].(*ast.KeyValueExpr)

	return ok
}

func (s *Struct) skippedPositional(count int, externalPkg bool) []Field {
	items := s.Fields.Items

	if count >= len(items) {
		return nil
	}

	remaining := items[count:]
	missing := make([]Field, 0, len(remaining))

	for _, f := range remaining {
		if s.isFieldRequired(f, externalPkg) {
			missing = append(missing, f)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	return missing
}

func (s *Struct) skippedNamed(lit *ast.CompositeLit, callerPkgPath string, canNamePromoted bool) []Field {
	groups := s.Fields.groupKeys(lit, callerPkgPath, canNamePromoted)

	// Capacity is a hint, not a contract: a key may name a field absent from
	// Items, since unexported fields are pruned for types whose fields come
	// from another package. Clamp so a mismatch can never turn into a makeslice
	// panic.
	missing := make([]Field, 0, max(len(s.Fields.Items)-groups.namedCount(), 0))

	missing = s.skippedIn(&s.Fields, groups, callerPkgPath, canNamePromoted, missing)

	if len(missing) == 0 {
		return nil
	}

	return missing
}

// skippedIn returns the required fields of fs the literal behind groups leaves
// uninitialized.
//
// Since Go 1.27 a composite literal may name a promoted field in place of the
// embedded field carrying it (golang/go#77245). Naming both a promoted field
// and its enclosing embedded field does not compile, so an embedded field is
// either named directly, partially filled through promoted names -- in which
// case its own missing fields are reported, each nameable from the same literal
// -- or absent altogether and reported as one missing field.
func (s *Struct) skippedIn(
	fs *Fields,
	groups *keyGroups,
	callerPkgPath string,
	canNamePromoted bool,
	missing []Field,
) []Field {
	externalPkg := fs.PackagePath != callerPkgPath

	for i, f := range fs.Items {
		if groups.names(i) {
			continue
		}

		// A blank field has no name to write, so a keyed literal can never
		// initialize it and reporting it yields a finding nobody can act on.
		// Positional literals do supply a value for it, which skippedPositional
		// accounts for by keeping blank fields in Items.
		if f.Name == BlankFieldName {
			continue
		}

		if !s.isFieldRequired(f, externalPkg) {
			missing = s.skippedUnder(f, groups.child(i), callerPkgPath, externalPkg, canNamePromoted, missing)

			continue
		}

		if child := groups.child(i); child != nil {
			missing = s.skippedIn(f.Embedded, child, callerPkgPath, canNamePromoted, missing)

			continue
		}

		missing = append(missing, f)
	}

	return missing
}

// skippedUnder returns the required fields the literal behind groups leaves
// uninitialized under an embedded field nothing requires. externalPkg tells
// whether f itself is declared outside the caller's package.
//
// An embedded field the enclosing type made optional still carries fields that
// are enforced in their own right, and such a field outranks the type holding
// it. Descending finds them; the field itself stays unreported, since nothing
// requires it. A field made optional in its own right is not descended into:
// that excludes what it promotes along with it.
//
// Below Go 1.27 a literal cannot name a promoted field, so the embedded field
// is the one key that reaches what is enforced under it, and it is what is
// reported. Where that field is unexported and the caller external, no key
// reaches them at all.
func (s *Struct) skippedUnder(
	f Field,
	groups *keyGroups,
	callerPkgPath string,
	externalPkg bool,
	canNamePromoted bool,
	missing []Field,
) []Field {
	if f.Embedded == nil || f.optedOut() {
		return missing
	}

	if canNamePromoted {
		return s.skippedIn(f.Embedded, groups, callerPkgPath, canNamePromoted, missing)
	}

	if !f.unreachable(externalPkg) && s.requiresAnyBelow(f.Embedded, callerPkgPath) {
		return append(missing, f)
	}

	return missing
}

// requiresAnyBelow reports whether leaving the embedded field holding fs
// unwritten leaves a required field unwritten with it. Fields no key reaches
// count for nothing, as they do in the descent itself.
func (s *Struct) requiresAnyBelow(fs *Fields, callerPkgPath string) bool {
	externalPkg := fs.PackagePath != callerPkgPath

	for _, f := range fs.Items {
		if f.Name == BlankFieldName || f.unreachable(externalPkg) {
			continue
		}

		if s.isFieldRequired(f, externalPkg) {
			return true
		}

		if f.Embedded != nil && !f.optedOut() && s.requiresAnyBelow(f.Embedded, callerPkgPath) {
			return true
		}
	}

	return false
}

// keyGroups holds the keys of one literal arranged as the field tree reaches
// them: one node per embedded field a key passes through, and at each node the
// indices the keys stopping there name.
//
// A literal writes its keys flat, and the descent reads them one level at a
// time. Arranging them once is what keeps a level from sorting the same keys
// again at every step below it.
type keyGroups struct {
	named    map[int]bool
	children map[int]*keyGroups
}

// names reports whether a key names the field at index i of this level.
func (g *keyGroups) names(i int) bool {
	return g != nil && g.named[i]
}

// namedCount returns how many fields of this level a key names.
func (g *keyGroups) namedCount() int {
	if g == nil {
		return 0
	}

	return len(g.named)
}

// child returns the keys reaching through the embedded field at index i, or nil
// when the literal writes none.
func (g *keyGroups) child(i int) *keyGroups {
	if g == nil {
		return nil
	}

	return g.children[i]
}

// add records one key by the path of field indices that reaches it.
func (g *keyGroups) add(path []int) {
	for _, i := range path[:len(path)-1] {
		if g.children == nil {
			g.children = make(map[int]*keyGroups)
		}

		child := g.children[i]
		if child == nil {
			child = &keyGroups{named: nil, children: nil}
			g.children[i] = child
		}

		g = child
	}

	if g.named == nil {
		g.named = make(map[int]bool)
	}

	g.named[path[len(path)-1]] = true
}

// groupKeys arranges the keys of lit into the tree the descent reads. A key
// naming nothing the type resolves is dropped: it names a field pruned for
// being inaccessible, one another shadows, or none at all.
func (fs *Fields) groupKeys(lit *ast.CompositeLit, callerPkgPath string, canNamePromoted bool) *keyGroups {
	paths := fs.paths
	if paths == nil {
		// Fields built outside the processor carry no promotion index, so a key
		// names a field of this level or nothing.
		paths = directPaths(fs)
	}

	// Most literals name only fields of the level they are written at, so the
	// root is sized for all of them and the levels below allocate on demand.
	groups := &keyGroups{named: make(map[int]bool, len(lit.Elts)), children: nil}

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		// Below Go 1.27 only a direct field can be named, whatever the tree
		// behind the type promotes.
		path := paths[promotionKey(callerPkgPath, key.Name)]
		if len(path) == 1 || (canNamePromoted && len(path) > 1) {
			groups.add(path)
		}
	}

	return groups
}

// directPaths indexes the fields of fs by name, reaching through nothing.
func directPaths(fs *Fields) map[string][]int {
	paths := make(map[string][]int, len(fs.Items))

	for i := range fs.Items {
		key := promotionKey(fs.PackagePath, fs.Items[i].Name)
		if _, seen := paths[key]; !seen {
			paths[key] = []int{i}
		}
	}

	return paths
}

// optedOut reports whether f was made optional in its own right rather than by
// the type holding it. Such a field carries its subtree out of the check with
// it, where a type marked optional carries nothing: a field enforced under one
// still outranks it.
func (f Field) optedOut() bool {
	return f.Optional || f.PatternOptional
}

func (s *Struct) isFieldRequired(f Field, externalPkg bool) bool {
	// Reach outranks every directive and pattern below, which are written
	// without knowing which literal reads them.
	if f.unreachable(externalPkg) {
		return false
	}

	// explicit field directives win over everything
	if f.Enforced {
		return true
	}

	if f.Optional {
		return false
	}

	// A pattern that names the field in its own right outranks the settings of
	// the type holding it. One broad enough to select the type as well names no
	// field in particular, and is answered by s.IsEnforced/IsOptional below --
	// PatternEnforced and PatternOptional are already false for it.
	if f.PatternEnforced {
		return true
	}

	if f.PatternOptional {
		return false
	}

	// optionality can be inherited from the structure settings
	if s.IsOptional() {
		return false
	}

	return true
}

type Field struct {
	Name     string
	Exported bool //exhaustruct:optional
	Enforced bool //exhaustruct:optional
	Optional bool //exhaustruct:optional

	// PatternEnforced and PatternOptional record that a pattern names this
	// field in its own right, and not by way of the type holding it.
	PatternEnforced bool //exhaustruct:optional
	PatternOptional bool //exhaustruct:optional

	// Shadowed marks a promoted field a literal of the outermost type resolves
	// to another field of the same name, or to none at all. Such a field has no
	// key to write and nothing may require it.
	Shadowed bool //exhaustruct:optional

	// Embedded holds the fields promoted by an embedded struct field, which a
	// literal may name in place of this field since Go 1.27. Nil for every
	// other field, including embedded pointers — a literal cannot reach through
	// those.
	Embedded *Fields //exhaustruct:optional
}

// unreachable reports whether no literal can write f: an unexported field
// declared in another package, or a promoted field another one shadows.
func (f Field) unreachable(externalPkg bool) bool {
	return f.Shadowed || (externalPkg && !f.Exported)
}

// indexPromotion records the chain of field indices reaching every name a
// literal of the type holding fs may write, and marks every field no chain
// reaches.
//
// Go resolves a promoted name to its shallowest occurrence, so a deeper field
// of that name is out of reach, and a name two fields share at one depth
// resolves to nothing, leaving both out of reach. The walk starts at the
// outermost struct, which is the only level a literal writes keys at, and is
// why the index is not kept per level.
func indexPromotion(fs *Fields) map[string][]int {
	paths := make(map[string][]int, len(fs.Items))
	blocked := make(map[string]bool)

	for level := []promotionLevel{{fields: fs, path: nil}}; len(level) > 0; {
		atDepth := namesAtDepth(level)

		level = indexLevel(level, paths, blocked, atDepth)

		for name := range atDepth {
			blocked[name] = true
		}
	}

	return paths
}

// promotionLevel is one struct of the promotion walk, with the chain of field
// indices that reached it.
type promotionLevel struct {
	fields *Fields
	path   []int
}

// promotionKey names a field the way a literal's key resolves to it. An
// unexported name belongs to the package that wrote it, so the same spelling in
// two packages stands for two identifiers: neither shadows the other, and a
// literal reaches only the one its own package declared.
func promotionKey(pkgPath, name string) string {
	if token.IsExported(name) {
		return name
	}

	return pkgPath + "." + name
}

// namesAtDepth counts the fields of one promotion depth carrying each name.
func namesAtDepth(level []promotionLevel) map[string]int {
	counts := make(map[string]int)

	for _, l := range level {
		for _, item := range l.fields.Items {
			counts[promotionKey(l.fields.PackagePath, item.Name)]++
		}
	}

	return counts
}

// indexLevel records the reachable fields of one depth, marks the rest, and
// returns the structs embedded at that depth, which make up the next one.
func indexLevel(
	level []promotionLevel,
	paths map[string][]int,
	blocked map[string]bool,
	atDepth map[string]int,
) []promotionLevel {
	var next []promotionLevel

	for _, l := range level {
		for i := range l.fields.Items {
			item := &l.fields.Items[i]
			path := append(append(make([]int, 0, len(l.path)+1), l.path...), i)

			key := promotionKey(l.fields.PackagePath, item.Name)

			item.Shadowed = blocked[key] || atDepth[key] > 1
			if !item.Shadowed {
				paths[key] = path
			}

			if item.Embedded != nil {
				next = append(next, promotionLevel{fields: item.Embedded, path: path})
			}
		}
	}

	return next
}

// Fields is a collection of struct fields with shared package metadata.
// Items are in declaration order (required for positional literals).
type Fields struct {
	PackagePath string
	Items       []Field

	// paths maps every name a literal of the enclosing type may write to the
	// chain of field indices reaching it. Built once with the type's metadata
	// and set on the outermost Fields only, which is the level keys are written
	// at. Nil for Fields built outside the processor.
	paths map[string][]int //exhaustruct:optional
}

func FormatFieldNames(fields []Field) string {
	switch len(fields) {
	case 0:
		return ""
	case 1:
		return fields[0].Name
	}

	var b strings.Builder

	b.Grow(len(fields))
	b.WriteString(fields[0].Name)

	for _, s := range fields[1:] {
		b.WriteString(", ")
		b.WriteString(s.Name)
	}

	return b.String()
}
