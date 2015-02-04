package app

import (
	"sort"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

type Environment map[string]string

func NewEnvironment(envs ...map[string]string) Environment {
	if len(envs) == 1 {
		return envs[0]
	}
	out := make(Environment)
	for _, env := range envs {
		for k, v := range env {
			out[k] = v
		}
	}
	return out
}

func (e Environment) List() []kapi.EnvVar {
	env := []kapi.EnvVar{}
	for k, v := range e {
		env = append(env, kapi.EnvVar{
			Name:  k,
			Value: v,
		})
	}
	sort.Sort(sortedEnvVar(env))
	return env
}

type sortedEnvVar []kapi.EnvVar

func (m sortedEnvVar) Len() int           { return len(m) }
func (m sortedEnvVar) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m sortedEnvVar) Less(i, j int) bool { return m[i].Name < m[j].Name }

func JoinEnvironment(a, b []kapi.EnvVar) (out []kapi.EnvVar) {
	out = a
	for i := range b {
		exists := false
		for j := range a {
			if a[j].Name == b[i].Name {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		out = append(out, b[i])
	}
	return out
}
