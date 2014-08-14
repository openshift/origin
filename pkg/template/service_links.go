package template

import "fmt"

func (e *Env) Append(env *Env) {
	*e = append(*e, *env...)
}

func (e *Env) Exists(name string) bool {
	for _, env := range *e {
		if env.Name == name {
			return true
		}
	}
	return false
}

func (s *Service) Containers() []*Container {
	result := make([]*Container, len((*s).DeploymentConfig.Deployment.PodTemplate.Containers))

	for i, _ := range s.DeploymentConfig.Deployment.PodTemplate.Containers {
		result[i] = &s.DeploymentConfig.Deployment.PodTemplate.Containers[i]
	}

	return result
}

func (s *Service) ContainersEnv() []*Env {
	var result []*Env
	for _, c := range s.Containers() {
		result = append(result, &c.Env)
	}
	return result
}

func (s *Service) AddEnv(env Env) {
	fmt.Printf("s.Containers() %+v\n", s.Containers())
	for _, c := range s.Containers() {
		(*c).Env = append(c.Env, env...)
	}
}

func (p *Template) ServiceByName(name string) *Service {
	for i, _ := range p.Services {
		if p.Services[i].Name == name {
			return &p.Services[i]
		}
	}
	return nil
}

func (p *Template) ProcessServiceLinks() {
	var (
		fromService, toService *Service
	)

	for i, _ := range p.ServiceLinks {
		fromService = p.ServiceByName(p.ServiceLinks[i].From)
		if fromService == nil {
			fmt.Printf("ERROR: Invalid FROM service in links: %+v\n", p.ServiceLinks[i].From)
			continue
		}

		toService = p.ServiceByName(p.ServiceLinks[i].To)
		if toService == nil {
			fmt.Printf("ERROR: Invalid TO service in links: %+v\n", p.ServiceLinks[i].To)
			continue
		}

		toService.AddEnv(p.ServiceLinks[i].Export)
	}
}
