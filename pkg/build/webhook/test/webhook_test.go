package test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/build/webhook/bitbucket"
	"github.com/openshift/origin/pkg/build/webhook/generic"
	"github.com/openshift/origin/pkg/build/webhook/github"
	"github.com/openshift/origin/pkg/build/webhook/gitlab"
)

type FakeSecretsGetter struct {
	Getter kcoreclient.SecretInterface
}

func (g *FakeSecretsGetter) Secrets(namespace string) kcoreclient.SecretInterface {
	return g.Getter
}

type FakeSecretInterface struct {
	Secrets map[string]*kapi.Secret
}

func (f *FakeSecretInterface) Create(s *kapi.Secret) (*kapi.Secret, error) {
	return nil, nil
}

func (f *FakeSecretInterface) Update(*kapi.Secret) (*kapi.Secret, error) {
	return nil, nil
}

func (f *FakeSecretInterface) Delete(name string, options *metav1.DeleteOptions) error {
	return nil
}
func (f *FakeSecretInterface) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return nil
}
func (f *FakeSecretInterface) Get(name string, options metav1.GetOptions) (*kapi.Secret, error) {
	return f.Secrets[name], nil
}
func (f *FakeSecretInterface) List(opts metav1.ListOptions) (*kapi.SecretList, error) {
	return nil, nil
}
func (f *FakeSecretInterface) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}
func (f *FakeSecretInterface) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *kapi.Secret, err error) {
	return nil, nil
}

func newBuildSource(ref string) *buildapi.BuildSource {
	return &buildapi.BuildSource{
		Git: &buildapi.GitBuildSource{
			Ref: ref,
		},
	}
}

func newBuildConfig() *buildapi.BuildConfig {
	return &buildapi.BuildConfig{
		Spec: buildapi.BuildConfigSpec{
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					Type: buildapi.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildapi.WebHookTrigger{
						Secret: "secret101",
					},
				},
				{
					Type: buildapi.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildapi.WebHookTrigger{
						Secret:   "secret100",
						AllowEnv: true,
					},
				},
				{
					Type: buildapi.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildapi.WebHookTrigger{
						Secret: "secret102",
					},
				},
				{
					Type: buildapi.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildapi.WebHookTrigger{
						Secret: "secret201",
					},
				},
				{
					Type: buildapi.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildapi.WebHookTrigger{
						Secret: "secret200",
					},
				},
				{
					Type: buildapi.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildapi.WebHookTrigger{
						Secret: "secret202",
					},
				},
				{
					Type: buildapi.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildapi.WebHookTrigger{
						Secret: "secret301",
					},
				},
				{
					Type: buildapi.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildapi.WebHookTrigger{
						Secret: "secret300",
					},
				},
				{
					Type: buildapi.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildapi.WebHookTrigger{
						Secret: "secret302",
					},
				},
				{
					Type: buildapi.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildapi.WebHookTrigger{
						Secret: "secret401",
					},
				},
				{
					Type: buildapi.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildapi.WebHookTrigger{
						Secret: "secret400",
					},
				},
				{
					Type: buildapi.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildapi.WebHookTrigger{
						Secret: "secret402",
					},
				},
			},
		},
	}
}

func TestWebHookEventUnmatchedRef(t *testing.T) {
	buildSourceGit := newBuildSource("wrongref")
	refMatch := webhook.GitRefMatches("master", webhook.DefaultConfigRef, buildSourceGit)
	if refMatch {
		t.Errorf("Expected Event Ref to not match BuildConfig Git Ref")
	}
}

func TestWebHookEventMatchedRef(t *testing.T) {
	buildSourceGit := newBuildSource("master")
	refMatch := webhook.GitRefMatches("master", webhook.DefaultConfigRef, buildSourceGit)
	if !refMatch {
		t.Errorf("Expected WebHook Event Ref to match BuildConfig Git Ref")
	}
}

func TestWebHookEventNoRef(t *testing.T) {
	buildSourceGit := newBuildSource("")
	refMatch := webhook.GitRefMatches("master", webhook.DefaultConfigRef, buildSourceGit)
	if !refMatch {
		t.Errorf("Expected WebHook Event Ref to match BuildConfig Git Ref")
	}
}

func TestFindTriggerPolicyWebHookError(t *testing.T) {
	buildConfig := &buildapi.BuildConfig{}
	plugins := []webhook.Plugin{
		&generic.WebHookPlugin{},
		&bitbucket.WebHookPlugin{},
		&gitlab.WebHookPlugin{},
		&github.WebHookPlugin{},
	}
	for _, p := range plugins {
		_, err := p.GetTriggers(buildConfig)
		if err != webhook.ErrHookNotEnabled {
			t.Errorf("Expected error %s got %s for plugin %#v", webhook.ErrHookNotEnabled, err, p)
		}
	}
}

func TestFindTriggerPolicyMatchedGenericWebHook(t *testing.T) {
	buildConfig := newBuildConfig()

	p := &generic.WebHookPlugin{}
	triggers, err := p.GetTriggers(buildConfig)

	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}

	if triggers == nil {
		t.Error("Expected a slice of matched 'triggers', got nil")
	}

	if len(triggers) != 3 {
		t.Errorf("Expected a slice of 3 matched triggers, got %d", len(triggers))
	}
}

func TestFindTriggerPolicyMatchedGithubWebHook(t *testing.T) {
	buildConfig := newBuildConfig()
	p := &github.WebHookPlugin{}
	triggers, err := p.GetTriggers(buildConfig)

	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}

	if triggers == nil {
		t.Error("Expected a slice of matched 'triggers', got nil")
	}

	if len(triggers) != 3 {
		t.Errorf("Expected a slice of 3 matched triggers, got %d", len(triggers))
	}
}

func TestFindTriggerPolicyMatchedGitLabWebHook(t *testing.T) {
	buildConfig := newBuildConfig()
	p := &gitlab.WebHookPlugin{}
	triggers, err := p.GetTriggers(buildConfig)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}

	if triggers == nil {
		t.Error("Expected a slice of matched 'triggers', got nil")
	}

	if len(triggers) != 3 {
		t.Errorf("Expected a slice of 3 matched triggers, got %d", len(triggers))
	}
}

func TestFindTriggerPolicyMatchedBitbucketWebHook(t *testing.T) {
	buildConfig := newBuildConfig()
	p := &bitbucket.WebHookPlugin{}
	triggers, err := p.GetTriggers(buildConfig)

	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}

	if triggers == nil {
		t.Error("Expected a slice of matched 'triggers', got nil")
	}

	if len(triggers) != 3 {
		t.Errorf("Expected a slice of 3 matched triggers, got %d", len(triggers))
	}
}

func TestValidateWrongWebHookSecretError(t *testing.T) {
	buildConfig := newBuildConfig()
	p := &generic.WebHookPlugin{}
	triggers, err := p.GetTriggers(buildConfig)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}
	_, err = webhook.CheckSecret("", "wrongsecret", triggers, nil)
	if err != webhook.ErrSecretMismatch {
		t.Errorf("Expected error %s, got %s", webhook.ErrSecretMismatch, err)
	}
}

func TestValidateMatchGenericWebHookSecret(t *testing.T) {
	secret := "secret101"
	buildConfig := newBuildConfig()
	p := &generic.WebHookPlugin{}
	triggers, err := p.GetTriggers(buildConfig)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}
	trigger, err := webhook.CheckSecret("", secret, triggers, nil)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}
	if trigger.Secret != secret {
		t.Errorf("Expected returned 'secret'(%s) to match %s", trigger.Secret, secret)
	}

	if trigger.AllowEnv {
		t.Errorf("Expected AllowEnv to be false for %s", secret)
	}
}

func TestValidateMatchGitHubWebHookSecret(t *testing.T) {
	secret := "secret201"
	buildConfig := newBuildConfig()
	p := &github.WebHookPlugin{}
	triggers, err := p.GetTriggers(buildConfig)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}
	trigger, err := webhook.CheckSecret("", secret, triggers, nil)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}

	if trigger.Secret != secret {
		t.Errorf("Expected returned 'secret'(%s) to match %s", trigger.Secret, secret)
	}

	if trigger.AllowEnv {
		t.Errorf("Expected AllowEnv to be false for %s", secret)
	}
}

func TestValidateMatchGitLabWebHookSecret(t *testing.T) {
	secret := "secret301"
	buildConfig := newBuildConfig()
	p := &gitlab.WebHookPlugin{}
	triggers, err := p.GetTriggers(buildConfig)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}
	trigger, err := webhook.CheckSecret("", secret, triggers, nil)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}

	if trigger.Secret != secret {
		t.Errorf("Expected returned 'secret'(%s) to match %s", trigger.Secret, secret)
	}

	if trigger.AllowEnv {
		t.Errorf("Expected AllowEnv to be false for %s", secret)
	}
}

func TestValidateMatchBitbucketWebHookSecret(t *testing.T) {
	secret := "secret401"
	buildConfig := newBuildConfig()
	p := &bitbucket.WebHookPlugin{}
	triggers, err := p.GetTriggers(buildConfig)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}
	trigger, err := webhook.CheckSecret("", secret, triggers, nil)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}

	if trigger.Secret != secret {
		t.Errorf("Expected returned 'secret'(%s) to match %s", trigger.Secret, secret)
	}

	if trigger.AllowEnv {
		t.Errorf("Expected AllowEnv to be false for %s", secret)
	}
}

func TestValidateEnvVarsGenericWebHook(t *testing.T) {
	secret := "secret100"
	buildConfig := newBuildConfig()
	p := &generic.WebHookPlugin{}
	triggers, err := p.GetTriggers(buildConfig)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}
	trigger, err := webhook.CheckSecret("", secret, triggers, nil)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}

	if trigger.Secret != secret {
		t.Errorf("Expected returned 'secret'(%s) to match %s", trigger.Secret, secret)
	}

	if !trigger.AllowEnv {
		t.Errorf("Expected AllowEnv to be true for %s", secret)
	}
}

func TestCheckSecret(t *testing.T) {
	t1 := &buildapi.WebHookTrigger{
		Secret: "secret1",
	}
	t2 := &buildapi.WebHookTrigger{
		Secret: "secret2",
	}
	m, err := webhook.CheckSecret("", "secret1", []*buildapi.WebHookTrigger{t1, t2}, nil)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}
	if m == nil {
		t.Errorf("Expected to match a trigger, got nil")
	}
	if m != t1 {
		t.Errorf("Expected to match trigger %v, matched trigger %v", *m, *t1)
	}

	m, err = webhook.CheckSecret("", "secret2", []*buildapi.WebHookTrigger{t1, t2}, nil)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}
	if m == nil {
		t.Errorf("Expected to match a trigger, got nil")
	}
	if m != t2 {
		t.Errorf("Expected to match trigger %v, matched trigger %v", *m, *t1)
	}

	m, err = webhook.CheckSecret("", "secret3", []*buildapi.WebHookTrigger{t1, t2}, nil)
	if err != webhook.ErrSecretMismatch {
		t.Errorf("Expected error %v, got %v", webhook.ErrSecretMismatch, err)
	}
	if m != nil {
		t.Errorf("Expected not to match a trigger, but matched %v", *m)
	}
}

func TestCheckSecretRef(t *testing.T) {
	secret1 := &kapi.Secret{
		Data: map[string][]byte{
			buildapi.WebHookSecretKey: []byte("secretvalue1"),
			"otherkey":                []byte("othersecretvalue"),
		},
	}
	secret2 := &kapi.Secret{
		Data: map[string][]byte{
			buildapi.WebHookSecretKey: []byte("secretvalue2"),
		},
	}
	invalidSecret := &kapi.Secret{
		Data: map[string][]byte{
			"somekey": []byte("secretvalue1"),
		},
	}
	getter := &FakeSecretInterface{
		Secrets: map[string]*kapi.Secret{
			"secret1":       secret1,
			"secret2":       secret2,
			"invalidSecret": invalidSecret,
		},
	}
	secretsClient := &FakeSecretsGetter{
		Getter: getter,
	}

	t1 := &buildapi.WebHookTrigger{
		SecretReference: &buildapi.SecretLocalReference{
			Name: "secret1",
		},
	}
	t2 := &buildapi.WebHookTrigger{
		SecretReference: &buildapi.SecretLocalReference{
			Name: "secret2",
		},
	}
	t3 := &buildapi.WebHookTrigger{
		SecretReference: &buildapi.SecretLocalReference{
			Name: "invalidSecret",
		},
	}
	m, err := webhook.CheckSecret("", "secretvalue1", []*buildapi.WebHookTrigger{t1, t2}, secretsClient)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}
	if m == nil {
		t.Errorf("Expected to match a trigger, got nil")
	}
	if m != t1 {
		t.Errorf("Expected to match trigger %v, matched trigger %v", *m, *t1)
	}

	m, err = webhook.CheckSecret("", "secretvalue2", []*buildapi.WebHookTrigger{t1, t2}, secretsClient)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}
	if m == nil {
		t.Errorf("Expected to match a trigger, got nil")
	}
	if m != t2 {
		t.Errorf("Expected to match trigger %v, matched trigger %v", *m, *t1)
	}

	m, err = webhook.CheckSecret("", "othersecretvalue", []*buildapi.WebHookTrigger{t1, t2}, secretsClient)
	if err != webhook.ErrSecretMismatch {
		t.Errorf("Expected error %v, got %v", webhook.ErrSecretMismatch, err)
	}
	if m != nil {
		t.Errorf("Expected not to match a trigger, but matched %v", *m)
	}

	m, err = webhook.CheckSecret("", "secretvalue1", []*buildapi.WebHookTrigger{t3}, secretsClient)
	if err != webhook.ErrSecretMismatch {
		t.Errorf("Expected error %v, got %v", webhook.ErrSecretMismatch, err)
	}
	if m != nil {
		t.Errorf("Expected not to match a trigger, but matched %v", *m)
	}
}
