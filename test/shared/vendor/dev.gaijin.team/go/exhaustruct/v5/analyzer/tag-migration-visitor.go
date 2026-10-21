package analyzer

import (
	"bytes"
	"go/ast"
	"go/token"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"dev.gaijin.team/go/exhaustruct/v5/internal/astutil"
	"dev.gaijin.team/go/exhaustruct/v5/internal/structure"
)

// tagMigrationVisitor scans struct definitions for deprecated exhaustruct tags
// and emits migration diagnostics with suggested fixes.
type tagMigrationVisitor struct {
	pass      *analysis.Pass
	processor *structure.Processor
	// sources caches the file bytes placement is read from.
	sources map[string][]byte
}

func newTagMigrationVisitor(pass *analysis.Pass, processor *structure.Processor) *tagMigrationVisitor {
	return &tagMigrationVisitor{
		pass:      pass,
		processor: processor,
		sources:   map[string][]byte{},
	}
}

func (v *tagMigrationVisitor) run() {
	insp := v.pass.ResultOf[inspect.Analyzer].(*inspector.Inspector) //nolint:forcetypeassert

	insp.Preorder([]ast.Node{new(ast.StructType)}, v.visitStructType)
}

func (v *tagMigrationVisitor) visitStructType(n ast.Node) {
	st, ok := n.(*ast.StructType)
	if !ok {
		return
	}

	if st.Fields == nil {
		return
	}

	for i, field := range st.Fields.List {
		if field.Tag == nil {
			continue
		}

		value, ok := parseExhaustructTag(field.Tag.Value)
		if !ok {
			continue
		}

		v.pass.Report(buildTagDiagnostic(field, v.placeTag(st, i), value))
	}
}

const (
	exhaustructTagKey = "exhaustruct"
	optionalTagValue  = "optional"
	optionalDirective = "//exhaustruct:optional"
	fixMessage        = "fix"
)

// parseExhaustructTag extracts value from an `exhaustruct:"value"` tag.
// Returns ("", false) if tag not present.
func parseExhaustructTag(tagLiteral string) (string, bool) {
	tag, err := strconv.Unquote(tagLiteral)
	if err != nil {
		return "", false
	}

	return reflect.StructTag(tag).Lookup(exhaustructTagKey)
}

func buildTagDiagnostic(field *ast.Field, placement tagPlacement, tagValue string) analysis.Diagnostic {
	var fixes []analysis.SuggestedFix

	// A name sharing its line with a block line directive takes no comment
	// above it: gofmt writes the two on lines of their own, which moves the
	// name off the line the generator pinned. Appending asks for no such
	// comment, so that placement is offered as it is anywhere else, and the
	// tag is reported without a fix only where neither is open to it.
	if placement.canAppend || !placement.movesALineDirective() {
		fixes = []analysis.SuggestedFix{buildTagFix(field, placement, tagValue)}
	}

	return analysis.Diagnostic{
		Pos:            field.Tag.Pos(),
		Message:        `struct tag "exhaustruct" is not supported anymore, use comment directives`,
		SuggestedFixes: fixes,
	}
}

func buildTagFix(field *ast.Field, placement tagPlacement, tagValue string) analysis.SuggestedFix {
	tag := field.Tag
	newTag := removeExhaustructFromTag(tag.Value)

	// Dropping the tag entirely takes the whitespace in front of it too, but
	// only whitespace: a comment written there carries its own meaning.
	start := tag.Pos()
	if newTag == "" && !placement.gapBeforeTag {
		start = field.Type.End()
	}

	edits := make([]analysis.TextEdit, 0, 1+len(placement.anchors))

	edits = append(edits, analysis.TextEdit{
		Pos:     start,
		End:     tag.End(),
		NewText: []byte(newTag),
	})

	if tagValue != optionalTagValue {
		return analysis.SuggestedFix{Message: fixMessage, TextEdits: edits}
	}

	// Where a line comment swallows nothing that matters, the directive takes
	// the tag's place. The tag ends the line the declaration starts on, so that
	// line holds every name it declares and the one anchor answers for all of
	// them.
	if placement.canAppend {
		if !placement.anchors[0].optionalityDecided {
			edits[0].NewText = []byte(appendedDirective(newTag, start == field.Type.End()))
		}

		return analysis.SuggestedFix{Message: fixMessage, TextEdits: edits}
	}

	// The directive takes a line of its own, and the name it precedes moves onto
	// the next. A name that already begins its line needs no break in front;
	// one sharing a line does, or the directive lands in the middle of it.
	for _, a := range placement.anchors {
		// A directive that already answers this name's optionality is the
		// author's own answer, and a newer one than the tag: the tag reached v5
		// inert, so removing it leaves that answer standing. Migrating on top of
		// it would contradict it, or repeat it into a group the scanner then
		// refuses to read.
		if a.optionalityDecided {
			continue
		}

		directiveLine := optionalDirective + "\n"
		if !a.startsLine {
			directiveLine = "\n" + directiveLine
		}

		edits = append(edits, analysis.TextEdit{
			Pos:     a.pos,
			End:     a.pos,
			NewText: []byte(directiveLine),
		})
	}

	return analysis.SuggestedFix{Message: fixMessage, TextEdits: edits}
}

// appendedDirective returns the text that takes a tag's place when the directive
// is appended: what is left of the tag, then the directive, parted by a space
// from whatever stands before it -- the tag's remainder, or the type where the
// whitespace in front of the tag went with the tag.
func appendedDirective(newTag string, afterType bool) string {
	if newTag != "" || afterType {
		return newTag + " " + optionalDirective
	}

	return optionalDirective
}

// tagPlacement records how a tagged field sits in its source. The visitor
// reads the bytes and the directives; the fix reads only the relations they
// answer.
type tagPlacement struct {
	// canAppend is true when the tag ends the line the field starts on, with
	// nothing but whitespace after it. A line comment written there runs to
	// the end of the line, so it takes anything else on it -- a field, the
	// closing brace, a comment, a semicolon -- into its own text. A tag ending
	// a later line of the field is no place for one either: a directive there
	// targets that line, which is not the field's.
	canAppend bool
	// anchors names the positions a directive is inserted above: one for each
	// line the declaration's names occupy, since field metadata is read at the
	// line of each name and one directive reaches only the line it is written
	// above.
	anchors []nameAnchor
	// gapBeforeTag is true when anything but whitespace stands between the
	// field's type and its tag.
	gapBeforeTag bool
}

// nameAnchor is a position a directive is inserted above, with whether the code
// there already begins its line and whether a directive there already answers
// the optionality of the name it precedes.
type nameAnchor struct {
	// pos is where the directive is written. It is the name's own position,
	// except where a line directive stands above the name: the directive then
	// goes in front of that line, which aboveLineDirective explains.
	pos        token.Pos
	startsLine bool
	// optionalityDecided records that a directive at the name already answers
	// its optionality, so the migration writes none.
	optionalityDecided bool
	// sharesLineDirective records that a block line directive stands on the
	// name's own line, which is what a comment written above the name moves it
	// past.
	sharesLineDirective bool
}

// movesALineDirective reports whether writing a directive above the names
// would move one off the line a line directive pinned it to.
func (p tagPlacement) movesALineDirective() bool {
	for _, a := range p.anchors {
		if a.sharesLineDirective && !a.optionalityDecided {
			return true
		}
	}

	return false
}

// nameAnchors lists one anchor for each line the declaration's names occupy.
// One declaration can spread them over several lines, and each name carries
// metadata of its own, down to the directive already written for it. A
// declaration with no name -- an embedded field -- is its own single anchor.
func (v *tagMigrationVisitor) nameAnchors(before token.Pos, field *ast.Field) []nameAnchor {
	fset := v.pass.Fset

	if len(field.Names) == 0 {
		return []nameAnchor{v.anchorAt(before, field.Pos())}
	}

	anchors := make([]nameAnchor, 0, len(field.Names))
	anchored := 0
	prev := before

	for _, name := range field.Names {
		if line := astutil.PhysicalLine(fset, name.Pos()); line != anchored {
			anchors = append(anchors, v.anchorAt(prev, name.Pos()))
			anchored = line
		}

		prev = name.End()
	}

	return anchors
}

// anchorAt reads the anchor for the name at pos, which prev is the code before.
//
// The scanner resolves a directive to the line it targets, which is the answer
// the checker itself acts on. Asking it, rather than the comments around the
// name, keeps the two from disagreeing about whose directive a comment is. Only
// optional and enforce answer optionality; an ignore reaching the line belongs
// to the statement or the type around it, and field metadata reads neither of
// those from it.
func (v *tagMigrationVisitor) anchorAt(prev, pos token.Pos) nameAnchor {
	fset := v.pass.Fset
	o := v.processor.Directives().LookupPos(fset, pos).Optionality()
	anchor, sharesLine := v.aboveLineDirective(pos)

	return nameAnchor{
		pos:                 anchor,
		startsLine:          astutil.PhysicalLine(fset, prev) != astutil.PhysicalLine(fset, pos),
		optionalityDecided:  o.Optional || o.Enforced,
		sharesLineDirective: sharesLine,
	}
}

// aboveLineDirective returns where a directive for the name at pos is written:
// the start of the line directive standing above it, where there is one, and
// pos itself otherwise.
//
// A line directive names the line that follows it. A directive written between
// the two takes that line for itself and moves the name onto the next, which
// renumbers what the generator pinned. Writing in front of the line directive
// leaves the numbering as it stands, and the two comments form one group, which
// the scanner reads for the code below it.
func (v *tagMigrationVisitor) aboveLineDirective(pos token.Pos) (anchor token.Pos, sharesLine bool) {
	fset := v.pass.Fset

	f := fset.File(pos)
	if f == nil {
		return pos, false
	}

	src := v.source(pos)
	if src == nil {
		return pos, false
	}

	line := astutil.PhysicalLine(fset, pos)
	lineStart := f.LineStart(line)

	// The block form can stand on the name's own line, in front of it. The name
	// then reads the line that directive names, and a comment written at the
	// name takes that line for itself and moves the name off it.
	if start, end := f.Offset(lineStart), f.Offset(pos); start >= 0 && end <= len(src) &&
		onlyLineDirective(src[start:end]) {
		return lineStart, true
	}

	// The first line of a file has nothing above it to read.
	const firstLine = 1

	if line <= firstLine {
		return pos, false
	}

	above := f.LineStart(line - 1)

	start, end := f.Offset(above), f.Offset(lineStart)
	if start < 0 || end > len(src) {
		return pos, false
	}

	// A line directive is read at the start of a line and nowhere else, so one
	// written further along renumbers nothing.
	if !isLineDirective(src[start:end]) {
		return pos, false
	}

	return above, false
}

// isLineDirective reports whether line opens with a directive that renumbers
// the source, in either of the two forms Go reads.
func isLineDirective(line []byte) bool {
	return bytes.HasPrefix(line, []byte("//line ")) || bytes.HasPrefix(line, []byte("/*line "))
}

// onlyLineDirective reports whether prefix holds a block line directive and
// nothing else but whitespace. Only that form can stand on the same line as the
// code it renumbers: the line form takes the rest of its line.
func onlyLineDirective(prefix []byte) bool {
	const opening, closing = "/*line ", "*/"

	trimmed := bytes.TrimLeft(prefix, " \t")
	if !bytes.HasPrefix(trimmed, []byte(opening)) {
		return false
	}

	_, after, found := bytes.Cut(trimmed[len(opening):], []byte(closing))
	if !found {
		return false
	}

	return len(bytes.TrimLeft(after, " \t")) == 0
}

// placeTag reads the placement of the field at index i.
func (v *tagMigrationVisitor) placeTag(st *ast.StructType, i int) tagPlacement {
	field := st.Fields.List[i]
	fset := v.pass.Fset

	before := st.Fields.Opening
	if i > 0 {
		before = st.Fields.List[i-1].End()
	}

	src := v.source(field.Tag.Pos())
	fieldLine := astutil.PhysicalLine(fset, field.Pos())

	return tagPlacement{
		canAppend: astutil.PhysicalLine(fset, field.Tag.End()) == fieldLine &&
			restOfLineBlank(fset, src, field.Tag.End()),
		anchors:      v.nameAnchors(before, field),
		gapBeforeTag: !spanBlank(fset, src, field.Type.End(), field.Tag.Pos()),
	}
}

// source returns the bytes of the file pos belongs to, or nil when they cannot
// be read. Every caller treats nil as "cannot tell" and takes its cautious
// branch.
func (v *tagMigrationVisitor) source(pos token.Pos) []byte {
	name := astutil.PhysicalFilename(v.pass.Fset, pos)
	if src, ok := v.sources[name]; ok {
		return src
	}

	src, err := v.pass.ReadFile(name)
	if err != nil {
		src = nil
	}

	v.sources[name] = src

	return src
}

// spanBlank reports whether the source between from and to holds nothing but
// whitespace. The scan stops at the first byte that is not, so asking about
// the rest of a line costs the gap in front of whatever comes next on it.
func spanBlank(fset *token.FileSet, src []byte, from, to token.Pos) bool {
	f := fset.File(from)
	if f == nil || src == nil || to < from {
		return false
	}

	start, end := f.Offset(from), f.Offset(to)
	if start < 0 || end > len(src) {
		return false
	}

	for _, b := range src[start:end] {
		if !isGoSpace(b) {
			return false
		}
	}

	return true
}

// isGoSpace reports whether b is one of the four bytes Go reads as whitespace
// between tokens.
func isGoSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}

// restOfLineBlank reports whether nothing but whitespace follows pos on its
// line.
func restOfLineBlank(fset *token.FileSet, src []byte, pos token.Pos) bool {
	f := fset.File(pos)
	if f == nil {
		return false
	}

	// LineStart counts the lines of the file on disk, and PhysicalLine answers
	// in the same count, so the number reads back into it.
	if line := astutil.PhysicalLine(fset, pos); line < f.LineCount() {
		return spanBlank(fset, src, pos, f.LineStart(line+1))
	}

	return spanBlank(fset, src, pos, f.Pos(f.Size()))
}

func removeExhaustructFromTag(tagLiteral string) string {
	tag, err := strconv.Unquote(tagLiteral)
	if err != nil {
		return tagLiteral
	}

	// Only the ASCII space separates entries: reflect skips exactly that byte
	// between them, and every byte above it belongs to the key it is reading.
	// Trimming by Unicode would take a non-breaking space off a key and rename
	// it.
	tag = strings.Trim(cutTagEntries(tag, exhaustructTagKey), " ")
	if tag == "" {
		return ""
	}

	// Struct tags are conventionally raw strings, but not every tag reaches
	// source as one. A backtick ends the literal, and an interpreted tag can
	// decode to bytes no raw literal may hold at all -- a NUL from \x00, which
	// the scanner rejects wherever it stands.
	if strconv.CanBackquote(tag) {
		return "`" + tag + "`"
	}

	return strconv.Quote(tag)
}

// cutTagEntries removes every entry named key from tag, leaving the rest byte
// for byte as it stands. It reads the grammar reflect.StructTag reads, so that
// removal and lookup agree on where an entry starts and ends. Lookup answers
// with the first of two identical keys, so a fix that stopped at that one
// would leave the field deprecated. The bytes past the point the tag stops
// parsing have no structure to cut along, and stay.
func cutTagEntries(tag, key string) string {
	var kept strings.Builder

	written := 0

	for i := 0; i < len(tag); {
		entryStart := i

		entryKey, quote, ok := scanTagKey(tag, i)
		if !ok {
			break
		}

		end, ok := scanTagValue(tag, quote)
		if !ok {
			break
		}

		i = end

		if entryKey == key {
			// The space in front of the entry belongs to it, and dropping the
			// entry without it would leave a gap twice as wide.
			kept.WriteString(tag[written:entryStart])

			written = i
		}
	}

	if written == 0 {
		return tag
	}

	kept.WriteString(tag[written:])

	return kept.String()
}

// scanTagKey returns the key of the entry starting at i, along with the index
// of the quote opening its value. It reports false when the bytes at i do not
// spell a key and the quote that opens its value.
func scanTagKey(tag string, i int) (key string, quote int, ok bool) {
	for i < len(tag) && tag[i] == ' ' {
		i++
	}

	start := i
	for i < len(tag) && isTagKeyByte(tag[i]) {
		i++
	}

	if i == start || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
		return "", 0, false
	}

	return tag[start:i], i + 1, true
}

// scanTagValue returns the index past the value that opens at quote. A
// backslash escapes the byte after it, so an escaped quote does not end the
// value.
func scanTagValue(tag string, quote int) (end int, ok bool) {
	for i := quote + 1; i < len(tag); i++ {
		if tag[i] == '"' {
			return i + 1, true
		}

		if tag[i] == '\\' {
			i++
		}
	}

	return 0, false
}

// isTagKeyByte reports whether b may appear in a tag key.
func isTagKeyByte(b byte) bool {
	return b > ' ' && b != ':' && b != '"' && b != 0x7f
}
