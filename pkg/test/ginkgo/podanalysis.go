package ginkgo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type PodElement struct {
	Namespace         string       `json:"namespace"`
	Kind              string       `json:"kind"`
	KindName          string       `json:"kindName"`
	PodName           string       `json:"podName"`
	Node              string       `json:"node"`
	CreationTimestamp metav1.Time  `json:"creationTimestamp"`
	DeletionTimestamp *metav1.Time `json:"deletionTimestamp"`
	Events            []string     `json:"events"`
}

func (pe *PodElement) String() string {
	return fmt.Sprintf("ns=%v, kind=%v, kindname=%v, podname=%v, node=%v, ct=%v, dt=%v", pe.Namespace, pe.Kind, pe.KindName, pe.PodName, pe.Node, pe.CreationTimestamp, pe.DeletionTimestamp)
}

func (pe *PodElement) KindOwnerKey() string {
	return fmt.Sprintf("%v/%v/%v", pe.Namespace, pe.Kind, pe.KindName)
}

func (pe *PodElement) UniqueKey() string {
	return fmt.Sprintf("%v/%v/%v/%v/%v", pe.Namespace, pe.Kind, pe.KindName, pe.PodName, pe.CreationTimestamp.Unix())
}

func NewPodCollector() *PodCollector {
	return &PodCollector{
		elements:         make(map[string][]*PodElement),
		podDisplacements: PodDisplacements{},
	}
}

func SetupNewPodCollector(ctx context.Context) (*PodCollector, error) {
	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
	clusterConfig, err := cfg.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("could not load client configuration: %v", err)
	}
	client, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return nil, err
	}

	pc := NewPodCollector()
	pc.Setup(ctx, informers.NewSharedInformerFactory(client, 10*time.Minute))
	return pc, nil
}

type Edge struct {
	In, Out *PodElement
}

type PodDisplacements map[string][][]Edge

func (pd PodDisplacements) Dump(minChainLen int) string {
	lines := []string{}
	for owner, chains := range pd {
		for _, chain := range chains {
			if len(chain) < minChainLen {
				continue
			}
			str := ""
			for idx, edge := range chain {
				if idx == 0 {
					str += fmt.Sprintf("\t%v(%v)%v [%v -> %v]", edge.In.PodName, edge.In.Node, edge.In.Events, edge.In.CreationTimestamp, edge.In.DeletionTimestamp)
				}
				str += fmt.Sprintf(" ->\n\t%v(%v)%v [%v -> %v]", edge.Out.PodName, edge.Out.Node, edge.Out.Events, edge.Out.CreationTimestamp, edge.Out.DeletionTimestamp)
			}
			lines = append(lines, fmt.Sprintf("%v (rescheduled=%v)\n%v", owner, len(chain), str))
		}
	}
	return strings.Join(lines, "\n")
}

type PodCollector struct {
	elements map[string][]*PodElement

	events []string

	podInformer cache.SharedIndexInformer

	// podDisplacements stores for each owner a list of pod replacements
	// (replacement = evicted/deleted pod replaced by a newly created one)
	podDisplacements PodDisplacements
}

func getPodElements(pod *corev1.Pod) (elements []*PodElement) {
	for _, owner := range pod.OwnerReferences {
		elements = append(elements, &PodElement{
			Namespace:         pod.Namespace,
			Kind:              owner.Kind,
			KindName:          owner.Name,
			PodName:           pod.Name,
			Node:              pod.Spec.NodeName,
			CreationTimestamp: pod.CreationTimestamp,
			DeletionTimestamp: pod.DeletionTimestamp,
		})
	}
	return
}

func (pc *PodCollector) Setup(ctx context.Context, sharedInformerFactory informers.SharedInformerFactory) {
	pc.podInformer = sharedInformerFactory.Core().V1().Pods().Informer()
	pc.podInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return
				}
				for _, element := range getPodElements(pod) {
					pc.Record(element)
				}
			},
			DeleteFunc: func(obj interface{}) {
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return
				}
				for _, element := range getPodElements(pod) {
					pc.Record(element)
				}
			},
		},
	)
}

func (pc *PodCollector) Run(ctx context.Context) {
	go pc.podInformer.Run(ctx.Done())
}

func (pc *PodCollector) SetEvents(events []string) {
	pc.events = events
}

func (pc *PodCollector) JsonDump() ([]byte, error) {
	bytes, err := json.Marshal(pc.elements)
	return bytes, err
}

func (pc *PodCollector) Import(data []byte) error {
	return json.Unmarshal(data, &pc.elements)
}

func (pc *PodCollector) Record(element *PodElement) {
	element.Events = append([]string{}, pc.events...)

	key := element.KindOwnerKey()
	pc.elements[key] = append(pc.elements[key], element)
}

func (pc *PodCollector) ComputePodTransitions() {
	pc.podDisplacements = PodDisplacements{}

	for key, podElements := range pc.elements {
		edges := map[string]*PodElement{}
		vertices := map[string]*PodElement{}
		notStart := map[string]struct{}{}
		for _, edge := range pc.computeKindOwnerPodTransitions(key, podElements) {
			vertices[edge.In.UniqueKey()] = edge.In
			vertices[edge.Out.UniqueKey()] = edge.Out
			notStart[edge.Out.UniqueKey()] = struct{}{}
			edges[edge.In.UniqueKey()] = edge.Out
		}

		for vertex := range edges {
			if _, exists := notStart[vertex]; exists {
				continue
			}
			placements := []Edge{}
			for {
				if _, exists := edges[vertex]; exists {
					placements = append(placements, Edge{In: vertices[vertex], Out: vertices[edges[vertex].UniqueKey()]})
					vertex = edges[vertex].UniqueKey()
				} else {
					break
				}
			}
			pc.podDisplacements[key] = append(pc.podDisplacements[key], placements)
		}
	}
}

func (pc *PodCollector) computeKindOwnerPodTransitions(kindOwner string, elements []*PodElement) (edges []Edge) {
	var sortedByCreationTimestamp []*PodElement
	var sortedByDeletionTimestamp []*PodElement

	// remove duplicates
	uniquePods := map[string]*PodElement{}
	for _, elm := range elements {
		if _, exists := uniquePods[elm.PodName]; !exists {
			uniquePods[elm.UniqueKey()] = elm
		} else {
			// Add missing DeletionTimestamp
			if uniquePods[elm.UniqueKey()].DeletionTimestamp == nil && elm.DeletionTimestamp != nil {
				uniquePods[elm.UniqueKey()] = elm
			}
		}
	}

	for _, elm := range uniquePods {
		sortedByCreationTimestamp = append(sortedByCreationTimestamp, elm)
		sortedByDeletionTimestamp = append(sortedByDeletionTimestamp, elm)
	}

	sort.Slice(sortedByCreationTimestamp, func(i, j int) bool {
		return sortedByCreationTimestamp[i].CreationTimestamp.Before(&sortedByCreationTimestamp[j].CreationTimestamp)
	})
	sort.Slice(sortedByDeletionTimestamp, func(i, j int) bool {
		if sortedByDeletionTimestamp[i].DeletionTimestamp == nil {
			return true
		}
		if sortedByDeletionTimestamp[j].DeletionTimestamp == nil {
			return false
		}
		return sortedByDeletionTimestamp[i].DeletionTimestamp.Before(sortedByDeletionTimestamp[j].DeletionTimestamp)
	})

	j := 0
	size := len(sortedByCreationTimestamp)
	for _, elm := range sortedByDeletionTimestamp {
		// No deletion ts -> no pod to append after
		if elm.DeletionTimestamp == nil {
			continue
		}
		for ; j < size; j++ {
			if sortedByCreationTimestamp[j].CreationTimestamp.Before(elm.DeletionTimestamp) {
				continue
			}
			break
		}
		if j < size {
			edges = append(edges, Edge{In: elm, Out: sortedByCreationTimestamp[j]})
		} else {
			return
		}
		j++
	}
	return
}

func (pc *PodCollector) PodDisplacements() PodDisplacements {
	return pc.podDisplacements
}
