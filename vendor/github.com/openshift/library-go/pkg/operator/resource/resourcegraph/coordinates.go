package resourcegraph

type ResourceCoordinates struct {
	Group     string
	Resource  string
	Namespace string
	Name      string
}

func (c ResourceCoordinates) String() string {
	resource := c.Resource
	if len(c.Group) > 0 {
		resource = resource + "." + c.Group
	}
	return resource + "/" + c.Name + "[" + c.Namespace + "]"
}
