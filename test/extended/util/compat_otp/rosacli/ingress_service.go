package rosacli

import (
	"bytes"

	logger "github.com/openshift/origin/test/extended/util/compat_otp/logext"
)

type IngressService interface {
	ResourcesCleaner

	EditIngress(clusterID string, ingressID string, flags ...string) (bytes.Buffer, error)
	ListIngress(clusterID string, flags ...string) (bytes.Buffer, error)
	DeleteIngress(clusterID string, ingressID string) (bytes.Buffer, error)
	ReflectIngressList(result bytes.Buffer) (res *IngressList, err error)
}

type ingressService struct {
	ResourcesService

	ingress map[string][]string
}

func NewIngressService(client *Client) IngressService {
	return &ingressService{
		ResourcesService: ResourcesService{
			client: client,
		},
		ingress: make(map[string][]string),
	}
}

func (i *ingressService) CleanResources(clusterID string) (errors []error) {
	var igsToDel []string
	igsToDel = append(igsToDel, i.ingress[clusterID]...)
	for _, igID := range igsToDel {
		logger.Infof("Remove remaining ingress '%s'", igID)
		_, err := i.DeleteIngress(clusterID, igID)
		if err != nil {
			errors = append(errors, err)
		}
	}

	return
}

// Struct for the 'rosa describe ingress' output
type IngressList struct {
	Ingresses []Ingress `json:"Ingresses,omitempty"`
}
type Ingress struct {
	ID                string `yaml:"ID,omitempty"`
	ApplicationRouter string `yaml:"APPLICATION ROUTER,omitempty"`
	Private           string `yaml:"PRIVATE,omitempty"`
	Default           string `yaml:"DEFAULT,omitempty"`
	RouteSelectors    string `yaml:"ROUTE SELECTORS,omitempty"`
	LBType            string `yaml:"LB-TYPE,omitempty"`
}

// Get specified ingress by ingress id
func (inl IngressList) Ingress(id string) (in *Ingress) {
	for _, inItem := range inl.Ingresses {
		if inItem.ID == id {
			in = &inItem
			return
		}
	}
	return
}

// Edit the cluster ingress
func (i *ingressService) EditIngress(clusterID string, ingressID string, flags ...string) (bytes.Buffer, error) {
	combflags := append([]string{"-c", clusterID}, flags...)
	editIngress := i.client.Runner.
		Cmd("edit", "ingress", ingressID).
		CmdFlags(combflags...)
	return editIngress.Run()
}

// List the cluster ingress
func (i *ingressService) ListIngress(clusterID string, flags ...string) (bytes.Buffer, error) {
	combflags := append([]string{"-c", clusterID}, flags...)
	listIngress := i.client.Runner.
		Cmd("list", "ingress").
		CmdFlags(combflags...)
	return listIngress.Run()
}

// Pasrse the result of 'rosa list ingress' to Ingress struct
func (i *ingressService) ReflectIngressList(result bytes.Buffer) (res *IngressList, err error) {
	res = &IngressList{}
	theMap := i.client.Parser.TableData.Input(result).Parse().Output()
	for _, ingressItem := range theMap {
		in := &Ingress{}
		err = MapStructure(ingressItem, in)
		if err != nil {
			return
		}
		res.Ingresses = append(res.Ingresses, *in)
	}
	return res, err
}

// Delete the ingress
func (i *ingressService) DeleteIngress(clusterID string, ingressID string) (output bytes.Buffer, err error) {
	output, err = i.client.Runner.
		Cmd("delete", "ingress", ingressID).
		CmdFlags("-c", clusterID, "-y").
		Run()
	if err == nil {
		i.ingress[clusterID] = RemoveFromStringSlice(i.ingress[clusterID], ingressID)
	}
	return
}
