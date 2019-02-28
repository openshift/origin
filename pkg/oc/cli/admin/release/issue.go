package release

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
)

var (
	reBugzilla    = regexp.MustCompile(`^https://bugzilla.redhat.com/(show_bug.cgi[?]id=)?(\d+)$`)
	reGitHubIssue = regexp.MustCompile(`^https://github.com/([^/]*)/([^/]*)/(issues|pull)/(\d+)$`)
)

type issue struct {
	// Store for the issue, e.g. "rhbz" for https://bugzilla.redhat.com, or "origin" for https://github.com/openshift/origin/issues.
	Store string

	// ID for the issue, e.g. 123.
	ID int

	// URI for the issue, e.g. https://bugzilla.redhat.com/show_bug.cgi?id=123.
	URI string
}

func (i *issue) Markdown() string {
	return fmt.Sprintf("[%s#%d](%s)", i.Store, i.ID, i.URI) // TODO: proper escaping
}

func issueFromURI(uri string) (*issue, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	issue := &issue{URI: uri}
	var id string

	switch parsed.Host {
	case "bugzilla.redhat.com":
		issue.Store = "rhbz"
		matches := reBugzilla.FindStringSubmatch(uri)
		if matches == nil {
			return nil, fmt.Errorf("could not extract Bugzilla bug ID from %q", uri)
		}

		id = matches[2]
	case "github.com":
		matches := reGitHubIssue.FindStringSubmatch(uri)
		if matches == nil {
			return nil, fmt.Errorf("could not extract GitHub issue ID from %q", uri)
		}

		org, repo := matches[1], matches[2]
		if org == "openshift" {
			issue.Store = repo
		} else {
			issue.Store = fmt.Sprintf("%s/%s", org, repo)
		}

		id = matches[4]
	default:
		return nil, fmt.Errorf("unrecognized issue host %q", parsed.Host)
	}

	issue.ID, err = strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("could not issue ID from %q: %v", uri, err)
	}

	return issue, nil
}
