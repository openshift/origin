package app

import (
	"testing"
)

func TestNewEnvironment(t *testing.T) {
	envs := Environment{
		"AA": "AAd",
		"BB": "BBd",
	}
	env := NewEnvironment(envs)
	if len(env) != 2 {
		t.Errorf("expected two items, got %d  %v", len(env), env)
	}
}

func TestAdd(t *testing.T) {
	envs := Environment{}
	env := NewEnvironment(envs)
	if len(env) != 0 {
		t.Errorf("expected two items, got %d  %v", len(env), env)
	}
	envs1 := Environment{
		"CC": "CCd",
	}
	env.Add(envs1)
	if len(env) != 1 {
		t.Errorf("expected 1 items, got %d  %v", len(env), env)
	}
}

func TestAddEnvVars(t *testing.T) {
	env := EnvVars{}
	env.Add("CC", "CCd")
	if len(env) != 1 {
		t.Errorf("expected 1 items, got %d  %v", len(env), env)
	}
}

func TestAddSecret(t *testing.T) {
	env := EnvVars{}
	env.AddSecret("CC", "CCd", "CCs")
	if len(env) != 1 {
		t.Errorf("expected 1 items, got %d %v", len(env), env)
	}
}

func TestListEnvars(t *testing.T) {
	env := EnvVars{}
	env.AddSecret("CC", "CCd", "CCs")
	env.Add("DD", "DDd")
	if len(env) != 2 {
		t.Errorf("expected 2 items, got %d %v", len(env), env)
	}
	envvars := env.List()
	if len(envvars) != 2 {
		t.Errorf("expected 2 items, got %d  %v", len(envvars), envvars)
	}

}
