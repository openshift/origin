package projects

import (
	"encoding/json"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/pagination"
)

// Option is a specific option defined at the API to enable features
// on a project.
type Option string

const (
	Immutable Option = "immutable"
)

type projectResult struct {
	gophercloud.Result
}

// GetResult is the result of a Get request. Call its Extract method to
// interpret it as a Project.
type GetResult struct {
	projectResult
}

// CreateResult is the result of a Create request. Call its Extract method to
// interpret it as a Project.
type CreateResult struct {
	projectResult
}

// DeleteResult is the result of a Delete request. Call its ExtractErr method to
// determine if the request succeeded or failed.
type DeleteResult struct {
	gophercloud.ErrResult
}

// UpdateResult is the result of an Update request. Call its Extract method to
// interpret it as a Project.
type UpdateResult struct {
	projectResult
}

// Project represents an OpenStack Identity Project.
type Project struct {
	// IsDomain indicates whether the project is a domain.
	IsDomain bool `json:"is_domain"`

	// Description is the description of the project.
	Description string `json:"description"`

	// DomainID is the domain ID the project belongs to.
	DomainID string `json:"domain_id"`

	// Enabled is whether or not the project is enabled.
	Enabled bool `json:"enabled"`

	// ID is the unique ID of the project.
	ID string `json:"id"`

	// Name is the name of the project.
	Name string `json:"name"`

	// ParentID is the parent_id of the project.
	ParentID string `json:"parent_id"`

	// Tags is the list of tags associated with the project.
	Tags []string `json:"tags,omitempty"`

	// Extra is free-form extra key/value pairs to describe the project.
	Extra map[string]interface{} `json:"-"`

	// Options are defined options in the API to enable certain features.
	Options map[Option]interface{} `json:"options,omitempty"`
}

func (r *Project) UnmarshalJSON(b []byte) error {
	type tmp Project
	var s struct {
		tmp
		Extra map[string]interface{} `json:"extra"`
	}
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	*r = Project(s.tmp)

	// Collect other fields and bundle them into Extra
	// but only if a field titled "extra" wasn't sent.
	if s.Extra != nil {
		r.Extra = s.Extra
	} else {
		var result interface{}
		err := json.Unmarshal(b, &result)
		if err != nil {
			return err
		}
		if resultMap, ok := result.(map[string]interface{}); ok {
			r.Extra = gophercloud.RemainingKeys(Project{}, resultMap)
		}
	}

	return err
}

// ProjectPage is a single page of Project results.
type ProjectPage struct {
	pagination.LinkedPageBase
}

// IsEmpty determines whether or not a page of Projects contains any results.
func (r ProjectPage) IsEmpty() (bool, error) {
	if r.StatusCode == 204 {
		return true, nil
	}

	projects, err := ExtractProjects(r)
	return len(projects) == 0, err
}

// NextPageURL extracts the "next" link from the links section of the result.
func (r ProjectPage) NextPageURL() (string, error) {
	var s struct {
		Links struct {
			Next     string `json:"next"`
			Previous string `json:"previous"`
		} `json:"links"`
	}
	err := r.ExtractInto(&s)
	if err != nil {
		return "", err
	}
	return s.Links.Next, err
}

// ExtractProjects returns a slice of Projects contained in a single page of
// results.
func ExtractProjects(r pagination.Page) ([]Project, error) {
	var s struct {
		Projects []Project `json:"projects"`
	}
	err := (r.(ProjectPage)).ExtractInto(&s)
	return s.Projects, err
}

// Extract interprets any projectResults as a Project.
func (r projectResult) Extract() (*Project, error) {
	var s struct {
		Project *Project `json:"project"`
	}
	err := r.ExtractInto(&s)
	return s.Project, err
}
