package partitioningbucketer

import (
	"fmt"
	"hash/fnv"
	"reflect"
	"sync"
	"time"

	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/util/sets"
)

type PartitioningBucketer struct {
	partitions []Partition
	keyFunc    KeyFunc

	workAvailable chan int
	stop          chan struct{}
}

type Partition struct {
	partitionId int
	lock        sync.Mutex
	parent      *PartitioningBucketer

	buckets   map[string]*Bucket
	workQueue chan string

	currentlyWorking sets.String
}

type Bucket struct {
	partitionId int
	key         string

	IncomingWorkChannel chan WorkItem
}

type WorkItem struct {
	Work interface{}

	ResponseChannel chan interface{}
}

type KeyFunc func(interface{}) string
type WorkFunc func(*Bucket)

func NamespaceKeyFunc(obj interface{}) string {
	fmt.Printf("#### looking for namespace in %v\n", reflect.TypeOf(obj))

	meta, err := meta.Accessor(obj)
	if err != nil {
		return ""
	}
	return meta.Namespace()
}

func NewPartitioningBucketer(numPartitions int, keyFunc KeyFunc, stop chan struct{}) *PartitioningBucketer {
	partitioningBucketer := &PartitioningBucketer{}

	partitions := []Partition{}
	for i := 0; i < numPartitions; i++ {
		partitions = append(partitions, *NewPartition(i, partitioningBucketer))
	}

	partitioningBucketer.partitions = partitions
	partitioningBucketer.keyFunc = keyFunc
	partitioningBucketer.stop = stop
	partitioningBucketer.workAvailable = make(chan int, 1000)

	return partitioningBucketer
}

func (p *PartitioningBucketer) AddWork(work interface{}, responseChannel chan interface{}) {
	key := p.keyFunc(work)
	partitionId := hash(key, uint32(len(p.partitions)))

	p.partitions[partitionId].AddWork(key, work, responseChannel)
}

func (p *PartitioningBucketer) DoWork(worker WorkFunc) {
	for {
		partitionId := -1
		select {
		case partitionId = <-p.workAvailable:
		case <-p.stop:
			return
		}

		p.partitions[partitionId].WorkBucket(worker)
	}
}

// hash calculates the hexadecimal representation (8-chars)
// of the hash of the passed in string using the FNV-a algorithm
func hash(s string, max uint32) int {
	hash := fnv.New32a()
	hash.Write([]byte(s))
	return int(hash.Sum32() % max)
}

func NewPartition(partitionId int, parent *PartitioningBucketer) *Partition {
	return &Partition{
		partitionId:      partitionId,
		parent:           parent,
		buckets:          map[string]*Bucket{},
		workQueue:        make(chan string, 100),
		currentlyWorking: sets.String{},
	}
}

func (p *Partition) AddWork(key string, work interface{}, responseChannel chan interface{}) {
	fmt.Printf("#### Starting %v!\n", key)

	workItem := WorkItem{
		Work:            work,
		ResponseChannel: responseChannel,
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	createdNew := false
	bucket, exists := p.buckets[key]
	if !exists {
		bucket = &Bucket{
			partitionId:         p.partitionId,
			key:                 key,
			IncomingWorkChannel: make(chan WorkItem, 100),
		}
		p.buckets[key] = bucket

		createdNew = true
	}

	bucket.IncomingWorkChannel <- workItem

	if createdNew {
		fmt.Printf("#### made new for %v!\n", key)
		p.workQueue <- key
		p.parent.workAvailable <- p.partitionId
	}
}

func (p *Partition) WorkBucket(worker WorkFunc) {
	bucket := p.getBucket()
	if bucket == nil {
		return
	}

	worker(bucket)

	// release the key
	p.lock.Lock()
	defer p.lock.Unlock()
	p.currentlyWorking.Delete(bucket.key)
}

func (p *Partition) getBucket() *Bucket {
	p.lock.Lock()
	defer p.lock.Unlock()

	key := ""
	select {
	case key = <-p.workQueue:

	case <-time.After(2 * time.Millisecond):
		return nil
	}

	// don't work on the same key in parallel, make callers wait for a bit to calm down
	// this effectively forces de-duping
	if p.currentlyWorking.Has(key) {
		time.Sleep(10 * time.Millisecond)
		p.workQueue <- key
		p.parent.workAvailable <- p.partitionId

		return nil
	}

	bucket, exists := p.buckets[key]
	if !exists {
		return nil
	}

	delete(p.buckets, key)

	p.currentlyWorking.Insert(key)
	return bucket
}
