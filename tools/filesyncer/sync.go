package main

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"
)

func main() {
	flushPoint := GlobalTracker.GetInspectionPoint("flush")
	syncPoint := GlobalTracker.GetInspectionPoint("sync")

	f, err := os.Create("filesyncer.txt")
	if err != nil {
		panic(err)
	}

	buffer := bufio.NewWriter(f)

	lastDuration := time.Duration(0)
	count := 0
	for {
		start := time.Now()
		message := fmt.Sprintf("%v    last cycle took %v seconds to complete\n", start, lastDuration.Seconds())

		if _, err := buffer.WriteString(message); err != nil {
			panic(err)
		}

		inst := flushPoint.StartInstance(fmt.Sprintf("%d", count))
		if err := buffer.Flush(); err != nil {
			panic(err)
		}
		flushPoint.EndInstance(inst.name)

		inst = syncPoint.StartInstance(fmt.Sprintf("%d", count))
		if err := f.Sync(); err != nil {
			panic(err)
		}
		syncPoint.EndInstance(inst.name)

		lastDuration = time.Since(start)
		count++
	}

}

var GlobalTracker = Tracker{inspectionPoints: map[string]*InspectionPoint{}}

type Tracker struct {
	inspectionPoints map[string]*InspectionPoint

	lock sync.RWMutex
}

type InspectionPoint struct {
	name string

	inspectionInstances map[string]*InspectionInstance

	// TODO find streaming stats library
	mean  time.Duration
	count int64

	nameCount     int64
	nameCountLock sync.Mutex

	notableInstances []*InspectionInstance
}

type InspectionInstance struct {
	name    string
	context interface{}

	stack    []byte
	start    *time.Time
	end      *time.Time
	duration *time.Duration
}

func (t *Tracker) GetInspectionPoint(name string) *InspectionPoint {
	t.lock.Lock()
	defer t.lock.Unlock()

	point, exists := t.inspectionPoints[name]
	if exists {
		return point
	}

	point, exists = t.inspectionPoints[name]
	if exists {
		return point
	}

	t.inspectionPoints[name] = &InspectionPoint{
		name:                name,
		inspectionInstances: map[string]*InspectionInstance{},
	}
	return t.inspectionPoints[name]
}

func (p *InspectionPoint) StartInstance(name string) *InspectionInstance {
	t := time.Now()

	if len(name) == 0 {
		p.nameCountLock.Lock()
		defer p.nameCountLock.Unlock()
		p.nameCount++

		name = fmt.Sprintf("%d", p.nameCount)
	}

	instance := &InspectionInstance{
		name:  name,
		start: &t,
	}

	p.inspectionInstances[name] = instance
	return instance
}

func (p *InspectionPoint) EndInstance(name string) {
	instance, exists := p.inspectionInstances[name]
	if !exists {
		return
	}

	if instance.start != nil && instance.end == nil {
		t := time.Now()
		instance.end = &t

		d := instance.end.Sub(*instance.start)
		instance.duration = &d

		if (p.count > 10) && *instance.duration > p.mean*10 {
			p.notableInstances = append(p.notableInstances, instance)

			// glog.Infof("#### InspectionPoint %s.%s (%v) took %v seconds instead of about %v from\n%s\n", p.name, instance.name, instance.context, instance.duration.Seconds(), p.mean.Seconds(), string(instance.stack))
			fmt.Printf("%v %s.%s (%v) took %v seconds instead of about %v\n", time.Now(), p.name, instance.name, instance.context, instance.duration.Seconds(), p.mean.Seconds())

		} else {
			// don't pollute the mean with outliers
			p.mean = time.Duration((int64(p.mean)*p.count + int64(*instance.duration)) / (p.count + 1))
			p.count++

		}
	}

	delete(p.inspectionInstances, name)
}

func (i *InspectionInstance) AddContext(context interface{}) {
	i.context = context
}
