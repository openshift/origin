package git

// CloneConfig specifies the options used when cloning the application source
// code.
type CloneConfig struct {
	Recursive bool
	Quiet     bool
}

// SourceInfo stores information about the source code
type SourceInfo struct {
	// Ref represents a commit SHA-1, valid Git branch name or a Git tag
	// The output image will contain this information as 'io.openshift.build.commit.ref' label.
	Ref string

	// CommitID represents an arbitrary extended object reference in Git as SHA-1
	// The output image will contain this information as 'io.openshift.build.commit.id' label.
	CommitID string

	// Date contains a date when the committer created the commit.
	// The output image will contain this information as 'io.openshift.build.commit.date' label.
	Date string

	// AuthorName contains the name of the author
	// The output image will contain this information (along with AuthorEmail) as 'io.openshift.build.commit.author' label.
	AuthorName string

	// AuthorEmail contains the e-mail of the author
	// The output image will contain this information (along with AuthorName) as 'io.openshift.build.commit.author' lablel.
	AuthorEmail string

	// CommitterName contains the name of the committer
	CommitterName string

	// CommitterEmail contains the e-mail of the committer
	CommitterEmail string

	// Message represents the first 80 characters from the commit message.
	// The output image will contain this information as 'io.openshift.build.commit.message' label.
	Message string

	// Location contains a valid URL to the original repository.
	// The output image will contain this information as 'io.openshift.build.source-location' label.
	Location string

	// ContextDir contains path inside the Location directory that
	// contains the application source code.
	// The output image will contain this information as 'io.openshift.build.source-context-dir'
	// label.
	ContextDir string
}
