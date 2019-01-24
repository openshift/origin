package status

import "sync"

type versionGetter struct {
	lock                 sync.Mutex
	versions             map[string]string
	notificationChannels []chan struct{}
}

func NewVersionGetter() VersionGetter {
	return &versionGetter{
		versions: map[string]string{},
	}
}

func (v *versionGetter) SetVersion(operandName, version string) {
	v.lock.Lock()
	defer v.lock.Unlock()

	v.versions[operandName] = version

	for i := range v.notificationChannels {
		ch := v.notificationChannels[i]
		// don't let a slow consumer block the rest
		go func() {
			ch <- struct{}{}
		}()
	}
}

func (v *versionGetter) GetVersions() map[string]string {
	v.lock.Lock()
	defer v.lock.Unlock()

	ret := map[string]string{}
	for k, v := range v.versions {
		ret[k] = v
	}
	return ret
}

func (v *versionGetter) VersionChangedChannel() <-chan struct{} {
	v.lock.Lock()
	defer v.lock.Unlock()

	channel := make(chan struct{}, 50)
	v.notificationChannels = append(v.notificationChannels, channel)
	return channel
}
