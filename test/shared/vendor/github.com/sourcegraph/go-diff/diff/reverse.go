package diff

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

// ErrCannotReverseCopy is returned when a Git copy diff cannot be reversed.
var ErrCannotReverseCopy = errors.New("cannot reverse a git copy diff")

// ReverseFileDiff takes a diff.FileDiff and returns the reverse operation.
// This is a FileDiff that undoes the edit of the original. Git copy diffs
// cannot be reversed because they do not contain enough information to delete
// the copied file.
func ReverseFileDiff(fd *FileDiff) (*FileDiff, error) {
	extended, err := reverseExtendedHeaders(fd.Extended, fd.OrigName, fd.NewName)
	if err != nil {
		return nil, err
	}
	reverse := FileDiff{
		OrigName: fd.NewName,
		OrigTime: fd.NewTime,
		NewName:  fd.OrigName,
		NewTime:  fd.OrigTime,
		Extended: extended,
	}
	for _, hunk := range fd.Hunks {
		invHunk, err := reverseHunk(hunk)
		if err != nil {
			return nil, err
		}
		reverse.Hunks = append(reverse.Hunks, invHunk)
	}
	return &reverse, nil
}

// reverseExtendedHeaders reverses the direction encoded in git's extended headers.
func reverseExtendedHeaders(headers []string, origName, newName string) ([]string, error) {
	parsed, ok := parseGitExtendedHeaders(headers)
	if !ok {
		return headers, nil
	}
	reversed := make([]string, len(headers))
	copy(reversed, headers)
	reversed[0] = reverseDiffGitHeader(reversed[0], origName, newName)
	for i, header := range parsed {
		switch header.kind {
		case gitExtendedHeaderNewFileMode:
			reversed[i] = gitExtendedHeaderDeletedFileMode + header.value()
		case gitExtendedHeaderDeletedFileMode:
			reversed[i] = gitExtendedHeaderNewFileMode + header.value()
		case gitExtendedHeaderIndex:
			reversed[i] = reverseIndexHeader(header)
		case gitExtendedHeaderCopyFrom, gitExtendedHeaderCopyTo:
			return nil, ErrCannotReverseCopy
		}
	}
	swapHeaderValues(reversed, parsed, gitModeHeaderPair)
	swapHeaderValues(reversed, parsed, gitRenameHeaderPair)
	return reversed, nil
}

// reverseDiffGitHeader swaps the two path arguments while preserving their
// original quoting. The parsed filenames are used only to reject malformed or
// ambiguous input; names recovered from other headers can disambiguate Git's
// unquoted paths containing spaces.
func reverseDiffGitHeader(header, origName, newName string) string {
	const prefix = gitExtendedHeaderDiff
	args := header[len(prefix):]
	lineEnding := ""
	if strings.HasSuffix(args, "\r") {
		args = strings.TrimSuffix(args, "\r")
		lineEnding = "\r"
	}

	first, second, valid := parseDiffGitArgs(args)
	if !valid {
		return header
	}

	var rawFirst, rawSecond string
	switch {
	case first != "" && second != "":
		var ok bool
		rawFirst, rawSecond, ok = splitDiffGitArgs(args, first, second)
		if !ok {
			return header
		}
	case origName != "" && newName != "" && args == origName+" "+newName:
		rawFirst, rawSecond = origName, newName
	default:
		return header
	}

	return prefix + rawSecond + " " + rawFirst + lineEnding
}

// splitDiffGitArgs locates the raw argument boundary after parseDiffGitArgs has
// validated and decoded both paths.
func splitDiffGitArgs(args, first, second string) (string, string, bool) {
	if args[0] == '"' {
		_, remainder, err := readQuotedFilename(args)
		if err != nil || len(remainder) < 2 || remainder[0] != ' ' {
			return "", "", false
		}
		return args[:len(args)-len(remainder)], remainder[1:], true
	}
	if args[len(args)-1] == '"' {
		i := strings.IndexByte(args, '"')
		if i < 2 || args[i-1] != ' ' {
			return "", "", false
		}
		return args[:i-1], args[i:], true
	}
	if args != first+" "+second {
		return "", "", false
	}
	return first, second, true
}

// swapHeaderValues exchanges the values of the first "from" header and the
// first "to" header, leaving both prefixes where they are.
func swapHeaderValues(headers []string, parsed gitExtendedHeaders, pair gitExtendedHeaderPair) {
	from, to, ok := parsed.pairIndices(pair)
	if !ok {
		return
	}
	headers[from] = pair.from + parsed[to].value()
	headers[to] = pair.to + parsed[from].value()
}

// reverseIndexHeader swaps the two blob hashes in an "index <old>..<new>[ <mode>]"
// header, leaving the trailing mode (if any) alone.
func reverseIndexHeader(header gitExtendedHeader) string {
	oldHash, newHash, ok := strings.Cut(header.value(), "..")
	if !ok || strings.ContainsAny(oldHash, " \r") {
		return header.raw
	}
	prefix := gitExtendedHeaderIndex
	if i := strings.IndexAny(newHash, " \r"); i >= 0 {
		return prefix + newHash[:i] + ".." + oldHash + newHash[i:]
	}
	return prefix + newHash + ".." + oldHash
}

// ReverseMultiFileDiff reverses a series of FileDiffs.
func ReverseMultiFileDiff(fds []*FileDiff) ([]*FileDiff, error) {
	var reverse []*FileDiff
	for _, fd := range fds {
		r, err := ReverseFileDiff(fd)
		if err != nil {
			return nil, err
		}
		reverse = append(reverse, r)
	}
	return reverse, nil
}

// A subhunk represents a portion of a Hunk.Body, split into three sections.
// It consists of zero or more context lines, followed by zero or more orig
// lines and then zero or more new lines.
//
// Each line is stored WITHOUT its starting character, but with the newlines
// included.  The final entry in a section may be missing a trailing newline.
//
// A missing newline in orig is represented in a Hunk by OrigNoNewlineAt,
// but is represented here as a missing newline.
type contextLine struct {
	body []byte
	bare bool
}

type subhunk struct {
	context []contextLine
	orig    [][]byte
	new     [][]byte
}

// reverseHunk converts a Hunk into its reverse operation.
func reverseHunk(forward *Hunk) (*Hunk, error) {
	reverse := Hunk{
		OrigStartLine:   forward.NewStartLine,
		OrigLines:       forward.NewLines,
		OrigNoNewlineAt: 0, // we may change this below
		NewStartLine:    forward.OrigStartLine,
		NewLines:        forward.OrigLines,
		Section:         forward.Section,
		StartPosition:   forward.StartPosition,
	}
	subs, err := toSubhunks(forward)
	if err != nil {
		return nil, err
	}
	for _, sub := range subs {
		invSub := subhunk{
			context: sub.context,
			orig:    sub.new,
			new:     sub.orig,
		}
		for _, line := range invSub.context {
			if line.bare {
				reverse.Body = append(reverse.Body, line.body...)
				continue
			}
			reverse.Body = append(reverse.Body, ' ')
			reverse.Body = append(reverse.Body, line.body...)
		}
		for _, line := range invSub.orig {
			reverse.Body = append(reverse.Body, '-')
			reverse.Body = append(reverse.Body, line...)
		}
		if len(invSub.orig) > 0 && reverse.Body[len(reverse.Body)-1] != '\n' {
			// There was a missing newline in `orig`, which we encode in a
			// hunk with an offset.
			reverse.Body = append(reverse.Body, '\n')
			reverse.OrigNoNewlineAt = int32(len(reverse.Body))
		}
		for _, line := range invSub.new {
			reverse.Body = append(reverse.Body, '+')
			reverse.Body = append(reverse.Body, line...)
		}
	}
	return &reverse, nil
}

func extractContextLines(from *[]byte) []contextLine {
	var lines []contextLine
	for len(*from) > 0 {
		if (*from)[0] == '\n' {
			lines = append(lines, contextLine{body: []byte{'\n'}, bare: true})
			*from = (*from)[1:]
			continue
		}
		if (*from)[0] != ' ' {
			break
		}

		newline := bytes.IndexByte(*from, '\n')
		if newline < 0 {
			lines = append(lines, contextLine{body: (*from)[1:]})
			*from = nil
			continue
		}

		lines = append(lines, contextLine{body: (*from)[1 : newline+1]})
		*from = (*from)[newline+1:]
	}
	return lines
}

func extractLinesStartingWith(from *[]byte, startingWith byte) [][]byte {
	var lines [][]byte
	for len(*from) > 0 {
		if (*from)[0] != startingWith {
			break
		}

		newline := bytes.IndexByte(*from, '\n')
		if newline < 0 {
			lines = append(lines, (*from)[1:])
			*from = nil
			continue
		}

		lines = append(lines, (*from)[1:newline+1])
		*from = (*from)[newline+1:]
	}
	return lines
}

// Extracts the subhunks from a diff.Hunk.
//
// This groups a Hunk's buffer into one or more subhunks, matching the conditions
// of `subhunk` above.  This function groups, strips prefix characters, and strips
// a newline for `OrigNoNewlineAt` if necessary.
func toSubhunks(hunk *Hunk) ([]subhunk, error) {
	var body []byte = hunk.Body
	var subhunks []subhunk
	if len(body) == 0 {
		return nil, nil
	}
	for len(body) > 0 {
		sh := subhunk{
			context: extractContextLines(&body),
			orig:    extractLinesStartingWith(&body, '-'),
			new:     extractLinesStartingWith(&body, '+'),
		}
		if len(sh.context) == 0 && len(sh.orig) == 0 && len(sh.new) == 0 {
			// The first line didn't start with any expected prefix.
			return nil, fmt.Errorf("unexpected character %q at start of line", body[0])
		}
		subhunks = append(subhunks, sh)
	}
	if hunk.OrigNoNewlineAt > 0 {
		// The Hunk represents a missing newline at the end of an "orig" line with a
		// OrigNoNewlineAt index.  We represent it here as an actual missing newline.
		var lastSubhunk *subhunk = &subhunks[len(subhunks)-1]
		s := len(lastSubhunk.orig)
		if s == 0 {
			return nil, errors.New("inconsistent OrigNoNewlineAt in input")
		}
		var cut bool
		lastSubhunk.orig[s-1], cut = bytes.CutSuffix(lastSubhunk.orig[s-1], []byte("\n"))
		if !cut {
			return nil, errors.New("missing newline in input")
		}
	}
	return subhunks, nil
}
