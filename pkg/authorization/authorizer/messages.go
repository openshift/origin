package authorizer

import (
	"bytes"
	"text/template"

	"k8s.io/apiserver/pkg/authorization/authorizer"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
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
	apiGroupIfNotEmpty := "{{if len .GetAPIGroup }}.{{.GetAPIGroup}}{{end}}"
	resourceWithSubresourceIfNotEmpty := "{{if len .GetSubresource }}{{.GetResource}}/{{.GetSubresource}}{{else}}{{.GetResource}}{{end}}"

	messageResolver := &ForbiddenMessageResolver{
		namespacedVerbsToResourcesToForbiddenMessageMaker: map[string]map[string]ForbiddenMessageMaker{},
		rootScopedVerbsToResourcesToForbiddenMessageMaker: map[string]map[string]ForbiddenMessageMaker{},
		nonResourceURLForbiddenMessageMaker:               newTemplateForbiddenMessageMaker(`User "{{.GetUser.GetName}}" cannot "{{.GetVerb}}" on "{{.GetPath}}"`),
		defaultForbiddenMessageMaker:                      newTemplateForbiddenMessageMaker(`User "{{.GetUser.GetName}}" cannot "{{.GetVerb}}" "` + resourceWithSubresourceIfNotEmpty + apiGroupIfNotEmpty + `" with name "{{.GetName}}" in project "{{.GetNamespace}}"`),
	}

	// general messages
	messageResolver.addNamespacedForbiddenMessageMaker("create", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.GetUser.GetName}}" cannot create `+resourceWithSubresourceIfNotEmpty+apiGroupIfNotEmpty+` in project "{{.GetNamespace}}"`))
	messageResolver.addRootScopedForbiddenMessageMaker("create", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.GetUser.GetName}}" cannot create `+resourceWithSubresourceIfNotEmpty+apiGroupIfNotEmpty+` at the cluster scope`))
	messageResolver.addNamespacedForbiddenMessageMaker("get", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.GetUser.GetName}}" cannot get `+resourceWithSubresourceIfNotEmpty+apiGroupIfNotEmpty+` in project "{{.GetNamespace}}"`))
	messageResolver.addRootScopedForbiddenMessageMaker("get", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.GetUser.GetName}}" cannot get `+resourceWithSubresourceIfNotEmpty+apiGroupIfNotEmpty+` at the cluster scope`))
	messageResolver.addNamespacedForbiddenMessageMaker("list", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.GetUser.GetName}}" cannot list `+resourceWithSubresourceIfNotEmpty+apiGroupIfNotEmpty+` in project "{{.GetNamespace}}"`))
	messageResolver.addRootScopedForbiddenMessageMaker("list", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.GetUser.GetName}}" cannot list all `+resourceWithSubresourceIfNotEmpty+apiGroupIfNotEmpty+` in the cluster`))
	messageResolver.addNamespacedForbiddenMessageMaker("watch", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.GetUser.GetName}}" cannot watch `+resourceWithSubresourceIfNotEmpty+apiGroupIfNotEmpty+` in project "{{.GetNamespace}}"`))
	messageResolver.addRootScopedForbiddenMessageMaker("watch", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.GetUser.GetName}}" cannot watch all `+resourceWithSubresourceIfNotEmpty+apiGroupIfNotEmpty+` in the cluster`))
	messageResolver.addNamespacedForbiddenMessageMaker("update", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.GetUser.GetName}}" cannot update `+resourceWithSubresourceIfNotEmpty+apiGroupIfNotEmpty+` in project "{{.GetNamespace}}"`))
	messageResolver.addRootScopedForbiddenMessageMaker("update", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.GetUser.GetName}}" cannot update `+resourceWithSubresourceIfNotEmpty+apiGroupIfNotEmpty+` at the cluster scope`))
	messageResolver.addNamespacedForbiddenMessageMaker("delete", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.GetUser.GetName}}" cannot delete `+resourceWithSubresourceIfNotEmpty+apiGroupIfNotEmpty+` in project "{{.GetNamespace}}"`))
	messageResolver.addRootScopedForbiddenMessageMaker("delete", authorizationapi.ResourceAll, newTemplateForbiddenMessageMaker(`User "{{.GetUser.GetName}}" cannot delete `+resourceWithSubresourceIfNotEmpty+apiGroupIfNotEmpty+` at the cluster scope`))

	// project request rejection
	projectRequestDeny := projectRequestForbiddenTemplate
	if len(projectRequestDeny) == 0 {
		projectRequestDeny = DefaultProjectRequestForbidden
	}
	messageResolver.addRootScopedForbiddenMessageMaker("create", "projectrequests", newTemplateForbiddenMessageMaker(projectRequestDeny))

	// projects "get" request rejection
	messageResolver.addNamespacedForbiddenMessageMaker("get", "projects", newTemplateForbiddenMessageMaker(`User "{{.GetUser.GetName}}" cannot get project "{{.GetNamespace}}"`))

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

func (m *ForbiddenMessageResolver) MakeMessage(attrs authorizer.Attributes) (string, error) {
	if !attrs.IsResourceRequest() {
		return m.nonResourceURLForbiddenMessageMaker.MakeMessage(attrs)
	}

	messageMakerMap := m.namespacedVerbsToResourcesToForbiddenMessageMaker
	if len(attrs.GetNamespace()) == 0 {
		messageMakerMap = m.rootScopedVerbsToResourcesToForbiddenMessageMaker
	}

	resourcesToForbiddenMessageMaker, exists := messageMakerMap[attrs.GetVerb()]
	if !exists {
		resourcesToForbiddenMessageMaker, exists = messageMakerMap[authorizationapi.VerbAll]
		if !exists {
			return m.defaultForbiddenMessageMaker.MakeMessage(attrs)
		}
	}

	messageMaker, exists := resourcesToForbiddenMessageMaker[attrs.GetResource()]
	if !exists {
		messageMaker, exists = resourcesToForbiddenMessageMaker[authorizationapi.ResourceAll]
		if !exists {
			return m.defaultForbiddenMessageMaker.MakeMessage(attrs)
		}
	}

	specificMessage, err := messageMaker.MakeMessage(attrs)
	if err != nil {
		return m.defaultForbiddenMessageMaker.MakeMessage(attrs)
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

func (m templateForbiddenMessageMaker) MakeMessage(attrs authorizer.Attributes) (string, error) {
	buffer := &bytes.Buffer{}
	err := m.parsedTemplate.Execute(buffer, attrs)
	return buffer.String(), err
}
