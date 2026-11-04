package nodeprovisioninglatency

import (
	"context"
	"sort"
	"sync"
	"time"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// collectionSyncTimeout bounds how long StartCollection waits for the initial list of Machines
// and Nodes. Failing to sync only costs the initial list; the informers keep running.
const collectionSyncTimeout = 10 * time.Second

// collectionState accumulates what the informers observed. Records are keyed by UID and are
// never removed, because a Machine that is created and deleted inside a run still has to produce
// a row.
type collectionState struct {
	lock sync.Mutex

	machines       map[types.UID]*machineRecord
	machinesByName map[string]*machineRecord
	nodes          map[types.UID]*nodeRecord
	nodesByName    map[string]*nodeRecord
}

func newCollectionState() *collectionState {
	return &collectionState{
		machines:       map[types.UID]*machineRecord{},
		machinesByName: map[string]*machineRecord{},
		nodes:          map[types.UID]*nodeRecord{},
		nodesByName:    map[string]*nodeRecord{},
	}
}

func (c *collectionState) machineFor(m *machinev1beta1.Machine) *machineRecord {
	rec, ok := c.machines[m.UID]
	if !ok {
		rec = &machineRecord{UID: m.UID}
		c.machines[m.UID] = rec
		c.machinesByName[m.Name] = rec
	}
	return rec
}

func (c *collectionState) observeMachine(m *machinev1beta1.Machine) {
	if m == nil {
		return
	}
	c.lock.Lock()
	defer c.lock.Unlock()

	rec := c.machineFor(m)
	rec.Name = m.Name
	rec.Creation = m.CreationTimestamp.Time
	rec.RoleLabel = m.Labels[machineRoleLabel]
	rec.MachineSet = m.Labels[machineSetLabel]
	rec.InstanceType = instanceTypeFromMachine(m)
	if m.Status.NodeRef != nil {
		rec.NodeRefName = m.Status.NodeRef.Name
	}
	if m.Status.Phase != nil {
		rec.Phase = *m.Status.Phase
	}

	if cond := findCondition(m.Status.Conditions, machinev1beta1.InstanceExistsCondition); cond != nil && cond.Status == corev1.ConditionTrue {
		rec.instanceExistsTime = cond.LastTransitionTime.Time
		rec.hasInstanceExists = true
	}

	// Fallback for Machines that never surface the condition: the first time we see the
	// instance-backed phases. Only meaningful for Machines created during the run, which
	// buildRow enforces.
	if !rec.hasProvisionedObserved &&
		(rec.Phase == string(machinev1beta1.PhaseProvisioned) || rec.Phase == string(machinev1beta1.PhaseRunning)) {
		rec.provisionedObserved = time.Now()
		rec.hasProvisionedObserved = true
	}
}

func (c *collectionState) observeMachineDeleted(obj any) {
	m, ok := obj.(*machinev1beta1.Machine)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		if m, ok = tombstone.Obj.(*machinev1beta1.Machine); !ok {
			return
		}
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	rec := c.machineFor(m)
	if rec.Name == "" {
		rec.Name = m.Name
	}
	if rec.Creation.IsZero() {
		rec.Creation = m.CreationTimestamp.Time
	}
	rec.deleted = true
	rec.deletedTime = time.Now()
}

func (c *collectionState) nodeFor(n *corev1.Node) *nodeRecord {
	rec, ok := c.nodes[n.UID]
	if !ok {
		rec = &nodeRecord{UID: n.UID}
		c.nodes[n.UID] = rec
		c.nodesByName[n.Name] = rec
	}
	return rec
}

func (c *collectionState) observeNode(n *corev1.Node) {
	if n == nil {
		return
	}
	c.lock.Lock()
	defer c.lock.Unlock()

	rec := c.nodeFor(n)
	rec.Name = n.Name
	rec.Creation = n.CreationTimestamp.Time
	rec.RoleLabel = nodeRoleFromLabels(n.Labels)
	rec.machineAnnotation = n.Annotations[machineAnnotation]

	if cond := nodeReadyCondition(n); cond != nil && cond.Status == corev1.ConditionTrue {
		if !rec.hasReadyObserved {
			rec.readyObserved = time.Now()
			rec.hasReadyObserved = true
		}
		// Guard against a missing transition time so buildRow can fall back to the time the
		// informer observed Ready instead of recording a zero timestamp.
		if !rec.hasReady && !cond.LastTransitionTime.IsZero() {
			rec.readyTime = cond.LastTransitionTime.Time
			rec.hasReady = true
		}
	}
}

func (c *collectionState) observeNodeDeleted(obj any) {
	n, ok := obj.(*corev1.Node)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		if n, ok = tombstone.Obj.(*corev1.Node); !ok {
			return
		}
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	rec := c.nodeFor(n)
	if rec.Name == "" {
		rec.Name = n.Name
	}
	if rec.Creation.IsZero() {
		rec.Creation = n.CreationTimestamp.Time
	}
	rec.deleted = true
}

// rows correlates Machines with Nodes and produces one row per observed Machine, ordered by
// Machine name so the autodl file is stable across runs. Masters that predate the run are
// dropped: they are created by the installer, so their timestamps measure install time rather
// than Machine API provisioning.
func (c *collectionState) rows(beginning time.Time) []provisioningRow {
	c.lock.Lock()
	defer c.lock.Unlock()

	nodesByMachine := map[string]*nodeRecord{}
	for _, node := range c.nodes {
		if node.machineAnnotation == "" {
			continue
		}
		nodesByMachine[machineNameFromAnnotation(node.machineAnnotation)] = node
	}

	rows := make([]provisioningRow, 0, len(c.machines))
	for _, rec := range c.machines {
		var node *nodeRecord
		if rec.NodeRefName != "" {
			node = c.nodesByName[rec.NodeRefName]
		}
		if node == nil {
			node = nodesByMachine[rec.Name]
		}
		row := buildRow(rec, node, beginning)
		if installerCreatedMaster(row) {
			continue
		}
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].MachineName < rows[j].MachineName })
	return rows
}

// startInformers starts the Machine and Node watches and waits briefly for the initial list.
// Errors are intentionally not returned: collection problems must never fail the job.
func (m *nodeProvisioningLatency) startInformers(ctx context.Context) {
	machineListWatch := cache.NewListWatchFromClient(
		m.machineClient.MachineV1beta1().RESTClient(), "machines", machineAPINamespace, fields.Everything())
	machineInformer := cache.NewSharedIndexInformer(
		machineListWatch, &machinev1beta1.Machine{}, 0,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	if _, err := machineInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { m.state.observeMachine(obj.(*machinev1beta1.Machine)) },
		UpdateFunc: func(_, obj any) { m.state.observeMachine(obj.(*machinev1beta1.Machine)) },
		DeleteFunc: m.state.observeMachineDeleted,
	}); err != nil {
		m.logger.Printf("unable to add Machine event handler: %v", err)
		return
	}

	nodeInformer := informerscorev1.NewNodeInformer(m.kubeClient, 0, cache.Indexers{})
	if _, err := nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { m.state.observeNode(obj.(*corev1.Node)) },
		UpdateFunc: func(_, obj any) { m.state.observeNode(obj.(*corev1.Node)) },
		DeleteFunc: m.state.observeNodeDeleted,
	}); err != nil {
		m.logger.Printf("unable to add Node event handler: %v", err)
		return
	}

	go machineInformer.Run(ctx.Done())
	go nodeInformer.Run(ctx.Done())

	syncCtx, cancel := context.WithTimeout(ctx, collectionSyncTimeout)
	defer cancel()
	if !cache.WaitForCacheSync(syncCtx.Done(), machineInformer.HasSynced, nodeInformer.HasSynced) {
		m.logger.Printf("timed out waiting for Machine/Node informer cache sync; continuing with partial data")
	}
}

// listFinalState lists Machines and Nodes one last time so objects that were missed by the
// watch are still recorded. Any error is logged and ignored.
func (m *nodeProvisioningLatency) listFinalState(ctx context.Context) {
	if machines, err := m.machineClient.MachineV1beta1().Machines(machineAPINamespace).List(ctx,
		metav1.ListOptions{}); err != nil {
		m.logger.Printf("final Machine list failed: %v", err)
	} else {
		for i := range machines.Items {
			m.state.observeMachine(&machines.Items[i])
		}
	}

	if nodes, err := m.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err != nil {
		m.logger.Printf("final Node list failed: %v", err)
	} else {
		for i := range nodes.Items {
			m.state.observeNode(&nodes.Items[i])
		}
	}
}
