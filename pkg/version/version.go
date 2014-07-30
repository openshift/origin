package version

func Get() (major, minor, gitCommit string) {
	return "0", "1", commitFromGit
}
