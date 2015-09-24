package authorizer

import (
	"bytes"
	"text/template"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

const DefaultProjectRequestForbidden = "You may not request a new project via this API."

type ForbiddenMessageResolver struct {
	// TODO if these maps were map[string]map[sets.String]ForbiddenMessageMaker, we'd be able to handle cases where sets of resources wanted slightly different messages
	// unfortunately, maps don't support keys like that, requiring sets.String serialization and deserialization.
	namespacedVerbsToResourcesToForbiddenMessageMaker map[string]map[string]ForbiddenMessageMaker
	rootScopedVerbsToResourcesToForbiddenMessageMaker map[string]map[string]ForbiddenMessageMaker

	nonResourceURLForbiddenMessageMaker ForbiddenMessageMaker
	defaultForbiddenMessageMaker        ForbiddenMessageMaker
}

func NewForbiddenMessageResolver(projectRequestForbiddenTemplate string) *ForbiddenMessageResolver {
	messageResolver := &ForbiddenMessageResolver{
		namespacedVerbsToResourcesToForbiddenMessageMaker: map[string]map[string]ForbiddenMessageMaker{},
		rootScopedVerbsToResourcesToForbiddenMessageMaker: map[string]map[string]ForbiddenMessageMaker{},
		nonResourceURLForbiddenMessageMaker:               newTemplateForbiddenMessageMaker(`User "{{.User.GetName}}" cannot "{{.Attributes.GetVerb}}" on "{{.Attributes.GetURL}}"`),
		defaultForbiddenMessageMaker:                      newTemplateForbiddenMessageMaker(`User "{{.User.GetName}}" cannot "{{.Attributes.GetVerb}}" "{{.Attributes.GetResource}}" with name "{{.Attributes.GetResourceName}}" in project "{{.Namespace}}"`),
	}

	// general messages
	messageResolver.addNamespacedForbiddenMessageMaker("create", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.User.GetName}}" cannot create {{.Attributes.GetResource}} in project "{{.Namespace}}"`))
	messageResolver.addRootScopedForbiddenMessageMaker("create", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.User.GetName}}" cannot create {{.Attributes.GetResource}} at the cluster scope`))
	messageResolver.addNamespacedForbiddenMessageMaker("get", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.User.GetName}}" cannot get {{.Attributes.GetResource}} in project "{{.Namespace}}"`))
	messageResolver.addRootScopedForbiddenMessageMaker("get", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.User.GetName}}" cannot get {{.Attributes.GetResource}} at the cluster scope`))
	messageResolver.addNamespacedForbiddenMessageMaker("list", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.User.GetName}}" cannot list {{.Attributes.GetResource}} in project "{{.Namespace}}"`))
	messageResolver.addRootScopedForbiddenMessageMaker("list", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.User.GetName}}" cannot list all {{.Attributes.GetResource}} in the cluster`))
	messageResolver.addNamespacedForbiddenMessageMaker("update", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.User.GetName}}" cannot update {{.Attributes.GetResource}} in project "{{.Namespace}}"`))
	messageResolver.addRootScopedForbiddenMessageMaker("update", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.User.GetName}}" cannot update {{.Attributes.GetResource}} at the cluster scope`))
	messageResolver.addNamespacedForbiddenMessageMaker("delete", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.User.GetName}}" cannot delete {{.Attributes.GetResource}} in project "{{.Namespace}}"`))
	messageResolver.addRootScopedForbiddenMessageMaker("delete", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.User.GetName}}" cannot delete {{.Attributes.GetResource}} at the cluster scope`))

	// project request rejection
	projectRequestDeny := projectRequestForbiddenTemplate
	if len(projectRequestDeny) == 0 {
		projectRequestDeny = DefaultProjectRequestForbidden
	}
	messageResolver.addRootScopedForbiddenMessageMaker("create", "projectrequests", newTemplateForbiddenMessageMaker(projectRequestDeny))

	return messageResolver
}

func (m *ForbiddenMessageResolver) addNamespacedForbiddenMessageMaker(verb, resource string, messageMaker ForbiddenMessageMaker) {
	m.addForbiddenMessageMaker(m.namespacedVerbsToResourcesToForbiddenMessageMaker, verb, resource, messageMaker)
}

func (m *ForbiddenMessageResolver) addRootScopedForbiddenMessageMaker(verb, resource string, messageMaker ForbiddenMessageMaker) {
	m.addForbiddenMessageMaker(m.rootScopedVerbsToResourcesToForbiddenMessageMaker, verb, resource, messageMaker)
}

func (m *ForbiddenMessageResolver) addForbiddenMessageMaker(target map[string]map[string]ForbiddenMessageMaker, verb, resource string, messageMaker ForbiddenMessageMaker) {
	resourcesToForbiddenMessageMaker, exists := target[verb]
	if !exists {
		resourcesToForbiddenMessageMaker = map[string]ForbiddenMessageMaker{}
		target[verb] = resourcesToForbiddenMessageMaker
	}

	resourcesToForbiddenMessageMaker[resource] = messageMaker
}

func (m *ForbiddenMessageResolver) MakeMessage(ctx MessageContext) (string, error) {
	if ctx.Attributes.IsNonResourceURL() {
		return m.nonResourceURLForbiddenMessageMaker.MakeMessage(ctx)
	}

	messageMakerMap := m.namespacedVerbsToResourcesToForbiddenMessageMaker
	if len(ctx.Namespace) == 0 {
		messageMakerMap = m.rootScopedVerbsToResourcesToForbiddenMessageMaker
	}

	resourcesToForbiddenMessageMaker, exists := messageMakerMap[ctx.Attributes.GetVerb()]
	if !exists {
		resourcesToForbiddenMessageMaker, exists = messageMakerMap[authorizationapi.VerbAll]
		if !exists {
			return m.defaultForbiddenMessageMaker.MakeMessage(ctx)
		}
	}

	messageMaker, exists := resourcesToForbiddenMessageMaker[ctx.Attributes.GetResource()]
	if !exists {
		messageMaker, exists = resourcesToForbiddenMessageMaker[authorizationapi.ResourceAll]
		if !exists {
			return m.defaultForbiddenMessageMaker.MakeMessage(ctx)
		}
	}

	specificMessage, err := messageMaker.MakeMessage(ctx)
	if err != nil {
		return m.defaultForbiddenMessageMaker.MakeMessage(ctx)
	}

	return specificMessage, nil
}

type templateForbiddenMessageMaker struct {
	parsedTemplate *template.Template
}

func newTemplateForbiddenMessageMaker(text string) templateForbiddenMessageMaker {
	parsedTemplate := template.Must(template.New("").Parse(text))

	return templateForbiddenMessageMaker{parsedTemplate}
}

func (m templateForbiddenMessageMaker) MakeMessage(ctx MessageContext) (string, error) {
	buffer := &bytes.Buffer{}
	err := m.parsedTemplate.Execute(buffer, ctx)
	return string(buffer.Bytes()), err
}
