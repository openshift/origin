package template

import "testing"

func TestServiceLinks(t *testing.T) {
	projectTempl.ProcessParameters()
	projectTempl.ProcessServiceLinks()

	s := projectTempl.ServiceByName("frontend")
	for _, env := range s.ContainersEnv() {
		for _, export := range projectTempl.ServiceLinks[0].Export {
			if env.Exists(export.Name) == false {
				t.Errorf("Failed to export %s variable via serviceLinks to %s", export.Name, s.Name)
			}
		}
	}
}
