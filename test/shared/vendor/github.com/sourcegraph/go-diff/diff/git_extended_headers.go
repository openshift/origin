package diff

import "strings"

const (
	gitExtendedHeaderDiff            = "diff --git "
	gitExtendedHeaderOldMode         = "old mode "
	gitExtendedHeaderNewMode         = "new mode "
	gitExtendedHeaderNewFileMode     = "new file mode "
	gitExtendedHeaderDeletedFileMode = "deleted file mode "
	gitExtendedHeaderRenameFrom      = "rename from "
	gitExtendedHeaderRenameTo        = "rename to "
	gitExtendedHeaderCopyFrom        = "copy from "
	gitExtendedHeaderCopyTo          = "copy to "
	gitExtendedHeaderIndex           = "index "
	gitExtendedHeaderBinaryFiles     = "Binary files "
	gitExtendedHeaderBinaryPatch     = "GIT binary patch"
)

var knownGitExtendedHeaderKinds = [...]string{
	gitExtendedHeaderDiff,
	gitExtendedHeaderOldMode,
	gitExtendedHeaderNewMode,
	gitExtendedHeaderNewFileMode,
	gitExtendedHeaderDeletedFileMode,
	gitExtendedHeaderRenameFrom,
	gitExtendedHeaderRenameTo,
	gitExtendedHeaderCopyFrom,
	gitExtendedHeaderCopyTo,
	gitExtendedHeaderIndex,
	gitExtendedHeaderBinaryFiles,
	gitExtendedHeaderBinaryPatch,
}

type gitExtendedHeader struct {
	raw  string
	kind string
}

func (h gitExtendedHeader) value() string {
	return h.raw[len(h.kind):]
}

type gitExtendedHeaders []gitExtendedHeader

func parseGitExtendedHeaders(raw []string) (gitExtendedHeaders, bool) {
	if len(raw) == 0 || !strings.HasPrefix(raw[0], gitExtendedHeaderDiff) {
		return nil, false
	}

	headers := make(gitExtendedHeaders, len(raw))
	for i, line := range raw {
		headers[i].raw = line
		for _, kind := range knownGitExtendedHeaderKinds {
			if strings.HasPrefix(line, kind) {
				headers[i].kind = kind
				break
			}
		}
	}
	return headers, true
}

type gitExtendedHeaderPair struct {
	from string
	to   string
}

var (
	gitModeHeaderPair   = gitExtendedHeaderPair{gitExtendedHeaderOldMode, gitExtendedHeaderNewMode}
	gitRenameHeaderPair = gitExtendedHeaderPair{gitExtendedHeaderRenameFrom, gitExtendedHeaderRenameTo}
	gitCopyHeaderPair   = gitExtendedHeaderPair{gitExtendedHeaderCopyFrom, gitExtendedHeaderCopyTo}
)

func (h gitExtendedHeaders) hasKind(index int, kind string) bool {
	return index >= 0 && index < len(h) && h[index].kind == kind
}

func (h gitExtendedHeaders) hasPairAt(index int, pair gitExtendedHeaderPair) bool {
	return h.hasKind(index, pair.from) && h.hasKind(index+1, pair.to)
}

func (h gitExtendedHeaders) pairIndices(pair gitExtendedHeaderPair) (from, to int, ok bool) {
	from, to = -1, -1
	for i, header := range h {
		if from < 0 && header.kind == pair.from {
			from = i
		}
		if to < 0 && header.kind == pair.to {
			to = i
		}
	}
	return from, to, from >= 0 && to >= 0
}
