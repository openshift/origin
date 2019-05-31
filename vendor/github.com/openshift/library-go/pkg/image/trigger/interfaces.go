package trigger

// TagRetriever returns information about a tag, including whether it exists
// and the observed resource version of the object at the time the tag was loaded.
type TagRetriever interface {
	ImageStreamTag(namespace, name string) (ref string, rv int64, ok bool)
}
