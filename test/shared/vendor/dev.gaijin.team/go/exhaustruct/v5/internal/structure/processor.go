package structure

import (
	"go/token"
	"go/types"

	"dev.gaijin.team/go/exhaustruct/v5/internal/astutil"
	"dev.gaijin.team/go/exhaustruct/v5/internal/cache"
	"dev.gaijin.team/go/exhaustruct/v5/internal/directive"
	"dev.gaijin.team/go/exhaustruct/v5/internal/pattern"
)

// structKey identifies a struct type in the struct cache.
//
// A source position cannot serve as that identity. Export data is not required
// to carry a faithful one: gcimporter discards the column outright and clamps
// any line past 64Ki to 1, so distinct declarations inside one dependency file
// collapse onto a single token.Position. go/types objects, in contrast, are
// unique per declaration per loaded package.
type structKey struct {
	// name is the type's TypeName, nil for anonymous structs.
	name *types.TypeName
	// strct is the underlying struct type. It alone identifies an anonymous
	// struct, and guards against a shared TypeName for a named one.
	strct *types.Struct
	// callerPkg is set for anonymous structs only, whose metadata is derived
	// from the package that uses them. Named types are caller-independent and
	// share one entry across every package that references them.
	callerPkg *types.Package
}

type Processor struct {
	directives  *directive.Scanner
	fieldsCache *cache.Cache[*types.Struct, *structFields]
	structCache *cache.Cache[structKey, *Struct]

	enforce    pattern.List //exhaustruct:optional
	ignore     pattern.List //exhaustruct:optional
	optional   pattern.List //exhaustruct:optional
	allowEmpty pattern.List //exhaustruct:optional
}

type Option func(*Processor)

func WithEnforce(patterns pattern.List) Option {
	return func(p *Processor) { p.enforce = patterns }
}

func WithIgnore(patterns pattern.List) Option {
	return func(p *Processor) { p.ignore = patterns }
}

func WithOptional(patterns pattern.List) Option {
	return func(p *Processor) { p.optional = patterns }
}

func WithAllowEmpty(patterns pattern.List) Option {
	return func(p *Processor) { p.allowEmpty = patterns }
}

const cachePreallocSize = 64

func NewProcessor(directives *directive.Scanner, opts ...Option) *Processor {
	p := &Processor{
		directives:  directives,
		fieldsCache: cache.New[*types.Struct, *structFields](cachePreallocSize),
		structCache: cache.New[structKey, *Struct](cachePreallocSize),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Directives returns the directive scanner the processor was constructed
// with, shared so callers can resolve use-site directives against the same
// cache.
func (p *Processor) Directives() *directive.Scanner {
	return p.directives
}

// ResolveStruct returns Struct metadata for the given type.
// Type resolution (pointers, aliases) is done by the caller.
//
// Parameters:
//   - typeName: the type's TypeName, or nil for anonymous structs
//   - strct: the underlying struct type (required)
//   - pos: position of type definition (from analyzer's AST inspection)
//   - callerPkg: package context, used for anonymous struct path
func (p *Processor) ResolveStruct(
	fset *token.FileSet,
	typeName *types.TypeName,
	strct *types.Struct,
	pos token.Pos,
	callerPkg *types.Package,
) *Struct {
	if strct == nil {
		return nil
	}

	key := structKey{name: typeName, strct: strct, callerPkg: nil}
	if typeName == nil {
		key.callerPkg = callerPkg
	}

	// One entry per key, filled once: passes run concurrently, and two of them
	// racing to build the same type would each pay for it and each hold a
	// value the other does not.
	return p.structCache.GetOrSet(key, func() *Struct {
		s := p.buildStruct(typeName, astutil.PhysicalPosition(fset, pos), callerPkg)

		p.populateFields(fset, s, strct)
		p.resolveStructDirectives(fset, s)
		p.matchStructPatterns(s)

		return s
	})
}

// buildStruct creates Struct metadata from type info.
func (*Processor) buildStruct(typeName *types.TypeName, pos token.Position, callerPkg *types.Package) *Struct {
	if typeName != nil {
		pkg := typeName.Pkg()

		return &Struct{
			Name:        typeName.Name(),
			FullPath:    pkg.Path() + "." + typeName.Name(),
			PackageName: pkg.Name(),
			Position:    pos,
		}
	}

	// Anonymous struct
	return &Struct{
		Name:        AnonymousName,
		FullPath:    callerPkg.Path() + "." + AnonymousName,
		PackageName: callerPkg.Name(),
		Position:    pos,
	}
}

// getStructFields returns the raw field data of strct, from the cache when it
// is there.
func (p *Processor) getStructFields(fset *token.FileSet, strct *types.Struct) *structFields {
	if fields, ok := p.fieldsCache.Get(strct); ok {
		return fields
	}

	fields := p.resolveStructFields(fset, strct)

	p.fieldsCache.Set(strct, fields)

	return fields
}

func (p *Processor) resolveStructFields(fset *token.FileSet, strct *types.Struct) *structFields {
	result := &structFields{
		strct:       strct,
		packagePath: "",
		fields:      make([]fieldInfo, 0, strct.NumFields()),
	}

	for f := range strct.Fields() {
		if result.packagePath == "" && f.Pkg() != nil {
			result.packagePath = f.Pkg().Path()
		}

		field := fieldInfo{
			name:     f.Name(),
			exported: f.Exported(),
		}

		if f.Embedded() {
			if embedded, ok := embeddedStruct(f.Type()); ok {
				field.embedded = p.getStructFields(fset, embedded)
			}
		}

		if p.directives != nil {
			o := p.directives.LookupPos(fset, f.Pos()).Optionality()

			field.enforced = o.Enforced
			field.optional = o.Optional
		}

		result.fields = append(result.fields, field)
	}

	return result
}

func (p *Processor) populateFields(fset *token.FileSet, s *Struct, strct *types.Struct) {
	s.Fields = p.buildFields(s, p.getStructFields(fset, strct))

	s.Fields.paths = indexPromotion(&s.Fields)
}

// buildFields turns resolved field data into the Struct's own view of it,
// applying pattern matching and access filtering. Embedded structs are built
// too, since a literal may name their fields directly.
//
// Fields are external when declared in a different package than the struct type.
// This happens for derived types and aliases from external packages.
//
// Rationale behind that filtering is that noone except package that has declared
// the struct can access unexported fields, therefore we can simply filter them
// out to save up on storage. Usage of derived type from the package of structure
// definition is simply impossible since it will cause import cycle - thus, such
// filtering is safe. An unexported embedded field is the exception, since a
// literal reaches the exported fields it promotes.
//
// The walk runs level by level and opens each struct once, at the depth it is
// first reached and only where it is reached once. Go resolves a promoted name
// to its shallowest occurrence and to none at all where two occurrences share a
// depth, so every other reach promotes nothing a literal can write. Opening
// them all would cost one subtree per path through the embedding graph, which
// doubles with every layer that reaches the one below it twice.
func (p *Processor) buildFields(s *Struct, resolved *structFields) Fields {
	root, pending := p.levelFields(s, resolved)
	opened := map[*types.Struct]bool{resolved.strct: true}

	for level := attribute(&root, pending); len(level) > 0; {
		occurrences := make(map[*types.Struct]int, len(level))

		for _, e := range level {
			occurrences[e.resolved.strct]++
		}

		var next []embeddedField

		for _, e := range level {
			if opened[e.resolved.strct] || occurrences[e.resolved.strct] > 1 {
				continue
			}

			opened[e.resolved.strct] = true

			sub, below := p.levelFields(s, e.resolved)

			e.owner.Items[e.index].Embedded = &sub
			next = append(next, attribute(&sub, below)...)
		}

		for r := range occurrences {
			opened[r] = true
		}

		level = next
	}

	return root
}

// embeddedField is one embedded struct waiting to be opened, and the field of
// an already built level that holds it.
type embeddedField struct {
	owner    *Fields
	index    int
	resolved *structFields
}

// attribute names owner as the level the pending fields belong to.
func attribute(owner *Fields, pending []embeddedField) []embeddedField {
	for i := range pending {
		pending[i].owner = owner
	}

	return pending
}

// levelFields builds the fields of one struct, without opening the structs it
// embeds, which it returns instead.
func (p *Processor) levelFields(s *Struct, resolved *structFields) (Fields, []embeddedField) {
	fieldsExternal := resolved.packagePath != s.PackagePath()

	fields := Fields{
		PackagePath: resolved.packagePath,
		Items:       make([]Field, 0, len(resolved.fields)),
		paths:       nil,
	}

	var pending []embeddedField

	for _, sf := range resolved.fields {
		// An unexported field declared in another package can never be written
		// here. An embedded one is still kept: a literal reaches the exported
		// fields under it by promotion, and the embedded field itself stays
		// unreported, since nothing may require what nobody can write.
		if fieldsExternal && !sf.exported && sf.embedded == nil {
			continue
		}

		// Promoted fields are addressed by the name a literal of s writes, so
		// every level shares the outer type's path.
		fieldPath := s.FullPath + "#" + sf.name

		fields.Items = append(fields.Items, Field{
			Name:            sf.name,
			Exported:        sf.exported,
			Enforced:        sf.enforced,
			Optional:        sf.optional,
			PatternEnforced: p.enforce.MatchFullStringExcept(fieldPath, s.FullPath),
			PatternOptional: p.optional.MatchFullStringExcept(fieldPath, s.FullPath),
			Embedded:        nil,
		})

		if sf.embedded != nil {
			pending = append(pending, embeddedField{
				owner:    nil,
				index:    len(fields.Items) - 1,
				resolved: sf.embedded,
			})
		}
	}

	return fields, pending
}

// embeddedStruct returns the struct type an embedded field promotes fields
// from. Embedded pointers yield nothing: a composite literal cannot reach
// through them, so their fields are not promotable in this context.
func embeddedStruct(typ types.Type) (*types.Struct, bool) {
	strct, ok := types.Unalias(typ).Underlying().(*types.Struct)

	return strct, ok
}

func (p *Processor) resolveStructDirectives(fset *token.FileSet, s *Struct) {
	if p.directives == nil || !s.Position.IsValid() {
		return
	}

	dirs := p.directives.Lookup(fset, s.Position)

	s.Enforced = dirs.Contains(directive.Enforce)
	s.Ignored = dirs.Contains(directive.Ignore)
	s.Optional = dirs.Contains(directive.Optional)
}

func (p *Processor) matchStructPatterns(s *Struct) {
	s.PatternEnforced = p.enforce.MatchFullString(s.FullPath)
	s.PatternIgnored = p.ignore.MatchFullString(s.FullPath)
	s.PatternOptional = p.optional.MatchFullString(s.FullPath)
	s.AllowEmptyDecl = p.allowEmpty.MatchFullString(s.FullPath)
}

func (p *Processor) Stats() (hits, misses, size uint64) {
	return p.structCache.Stats()
}
