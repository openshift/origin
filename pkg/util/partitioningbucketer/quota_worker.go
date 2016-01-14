package partitioningbucketer

import (
	"errors"
	"fmt"
	"strings"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kresource "k8s.io/kubernetes/pkg/api/resource"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
)

type QuotaHandler struct {
	KubeClient *kclient.Client

	PartitionBucketer *PartitioningBucketer
}

func (h *QuotaHandler) Start(numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		go h.PartitionBucketer.DoWork(h.HandleQuotaBucket)
	}
}

func (h *QuotaHandler) HandleQuotaBucket(bucket *Bucket) {
	quotas, _ := h.KubeClient.ResourceQuotas(bucket.key).List(labels.Everything(), fields.Everything())
	quota := kapi.ResourceQuota{}

	projectedAccepts := []chan interface{}{}
	acceptVal := []string{}
	failureVal := []string{}

	for {
		if len(bucket.IncomingWorkChannel) == 0 {
			break
		}

		workItem := <-bucket.IncomingWorkChannel

		fmt.Printf("got %#v\n", workItem)
		pod := workItem.Work.(*kapi.Pod)

		if len(quotas.Items) < 1 {
			fmt.Printf("#### MISSING!!!!")
			acceptVal = append(acceptVal, fmt.Sprintf("accepted %v/%v ", pod.Namespace, pod.Name))
			failureVal = append(failureVal, fmt.Sprintf("failure %v/%v", pod.Namespace, pod.Name))
			projectedAccepts = append(projectedAccepts, workItem.ResponseChannel)
			continue
		}

		// just an example, pull the first one
		quota = quotas.Items[0]

		podLimit := quota.Spec.Hard[kapi.ResourcePods]
		maxPods := int((&podLimit).Value())

		podUsed := quota.Status.Used[kapi.ResourcePods]
		currPods := int((&podUsed).Value())
		projectedPods := currPods + 1
		if projectedPods > maxPods {
			rejection := fmt.Sprintf("rejected %v/%v curr=%v max=%v!", pod.Namespace, pod.Name, currPods, maxPods)
			workItem.ResponseChannel <- rejection
			fmt.Printf("#### Planning to %s\n", rejection)
			continue
		}
		quota.Status.Used[kapi.ResourcePods] = kresource.MustParse(fmt.Sprintf("%d", projectedPods))

		acceptVal = append(acceptVal, fmt.Sprintf("accepted %v/%v curr=%v max=%v!", pod.Namespace, pod.Name, currPods, maxPods))
		failureVal = append(failureVal, fmt.Sprintf("failure %v/%v curr=%v max=%v!", pod.Namespace, pod.Name, currPods, maxPods))
		projectedAccepts = append(projectedAccepts, workItem.ResponseChannel)

		fmt.Printf("#### Planning to %s\n", acceptVal)
	}

	fmt.Printf("#### About to update for %v with %#v\n", bucket.key, quota.Status.Used[kapi.ResourcePods])
	if _, err := h.KubeClient.ResourceQuotas(bucket.key).UpdateStatus(&quota); err == nil {
		for i, accept := range projectedAccepts {
			accept <- acceptVal[i]
		}

	} else {
		for i, accept := range projectedAccepts {
			fmt.Printf("#### FAILURE %v for %v", err, failureVal[i])
			accept <- failureVal[i]
		}
	}
}

func (h *QuotaHandler) Admit(a admission.Attributes) (err error) {

	responseChannel := make(chan interface{})

	if strings.Contains(strings.ToLower(a.GetResource()), "pod") {
		fmt.Printf("#### IN HANDLER with %v\n", a.GetName())

		h.PartitionBucketer.AddWork(a.GetObject(), responseChannel)

		select {
		case response := <-responseChannel:
			answer := response.(string)
			switch {
			case strings.HasPrefix(answer, "accepted"):
				fmt.Printf("#### ACCEPTED with %v\n", answer)
				return nil
			case strings.HasPrefix(answer, "rejected"):
				fmt.Printf("#### REJECTED with %v\n", answer)
				return admission.NewForbidden(a, errors.New(answer))
			case strings.HasPrefix(answer, "failure"):
				return errors.New(answer)

			}
		}
	}

	return errors.New("WTF!")
}

func (h *QuotaHandler) Handles(operation admission.Operation) bool {
	return true
}
