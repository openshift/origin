package auth

import (
	"errors"
	"sync"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/authentication/user"
	kstorage "k8s.io/apiserver/pkg/storage"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	projectcache "github.com/openshift/origin/pkg/project/cache"
	projectutil "github.com/openshift/origin/pkg/project/util"
)

type CacheWatcher interface {
	// GroupMembershipChanged is called serially for all changes for all watchers.  This method MUST NOT BLOCK.
	// The serial nature makes reasoning about the code easy, but if you block in this method you will doom all watchers.
	GroupMembershipChanged(namespaceName string, users, groups sets.String)
}

type WatchableCache interface {
	// RemoveWatcher removes a watcher
	RemoveWatcher(CacheWatcher)
	// List returns the set of namespace names the user has access to view
	List(userInfo user.Info) (*kapi.NamespaceList, error)
}

// userProjectWatcher converts a native etcd watch to a watch.Interface.
type userProjectWatcher struct {
	user user.Info
	// visibleNamespaces are the namespaces that the scopes allow
	visibleNamespaces sets.String

	// cacheIncoming is a buffered channel used for notification to watcher.  If the buffer fills up,
	// then the watcher will be removed and the connection will be broken.
	cacheIncoming chan watch.Event
	// cacheError is a cached channel that is put to serially.  In theory, only one item will
	// ever be placed on it.
	cacheError chan error

	// outgoing is the unbuffered `ResultChan` use for the watch.  Backups of this channel will block
	// the default `emit` call.  That's why cacheError is a buffered channel.
	outgoing chan watch.Event
	// userStop lets a user stop his watch.
	userStop chan struct{}

	// stopLock keeps parallel stops from doing crazy things
	stopLock sync.Mutex

	// Injectable for testing. Send the event down the outgoing channel.
	emit func(watch.Event)

	projectCache *projectcache.ProjectCache
	authCache    WatchableCache

	initialProjects []kapi.Namespace
	// knownProjects maps name to resourceVersion
	knownProjects map[string]string
}

var (
	// watchChannelHWM tracks how backed up the most backed up channel got.  This mirrors etcd watch behavior and allows tuning
	// of channel depth.
	watchChannelHWM kstorage.HighWaterMark
)

func NewUserProjectWatcher(user user.Info, visibleNamespaces sets.String, projectCache *projectcache.ProjectCache, authCache WatchableCache, includeAllExistingProjects bool) *userProjectWatcher {
	namespaces, _ := authCache.List(user)
	knownProjects := map[string]string{}
	for _, namespace := range namespaces.Items {
		knownProjects[namespace.Name] = namespace.ResourceVersion
	}

	// this is optional.  If they don't request it, don't include it.
	initialProjects := []kapi.Namespace{}
	if includeAllExistingProjects {
		initialProjects = append(initialProjects, namespaces.Items...)
	}

	w := &userProjectWatcher{
		user:              user,
		visibleNamespaces: visibleNamespaces,

		cacheIncoming: make(chan watch.Event, 1000),
		cacheError:    make(chan error, 1),
		outgoing:      make(chan watch.Event),
		userStop:      make(chan struct{}),

		projectCache:    projectCache,
		authCache:       authCache,
		initialProjects: initialProjects,
		knownProjects:   knownProjects,
	}
	w.emit = func(e watch.Event) {
		select {
		case w.outgoing <- e:
		case <-w.userStop:
		}
	}
	return w
}

func (w *userProjectWatcher) GroupMembershipChanged(namespaceName string, users, groups sets.String) {
	if !w.visibleNamespaces.Has("*") && !w.visibleNamespaces.Has(namespaceName) {
		// this user is scoped to a level that shouldn't see this update
		return
	}

	hasAccess := users.Has(w.user.GetName()) || groups.HasAny(w.user.GetGroups()...)
	_, known := w.knownProjects[namespaceName]

	switch {
	// this means that we were removed from the project
	case !hasAccess && known:
		delete(w.knownProjects, namespaceName)

		select {
		case w.cacheIncoming <- watch.Event{
			Type:   watch.Deleted,
			Object: projectutil.ConvertNamespace(&kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}),
		}:
		default:
			// remove the watcher so that we wont' be notified again and block
			w.authCache.RemoveWatcher(w)
			w.cacheError <- errors.New("delete notification timeout")
		}

	case hasAccess:
		namespace, err := w.projectCache.GetNamespace(namespaceName)
		if err != nil {
			utilruntime.HandleError(err)
			return
		}

		event := watch.Event{
			Type:   watch.Added,
			Object: projectutil.ConvertNamespace(namespace),
		}

		// if we already have this in our list, then we're getting notified because the object changed
		if lastResourceVersion, known := w.knownProjects[namespaceName]; known {
			event.Type = watch.Modified

			// if we've already notified for this particular resourceVersion, there's no work to do
			if lastResourceVersion == namespace.ResourceVersion {
				return
			}
		}
		w.knownProjects[namespaceName] = namespace.ResourceVersion

		select {
		case w.cacheIncoming <- event:
		default:
			// remove the watcher so that we won't be notified again and block
			w.authCache.RemoveWatcher(w)
			w.cacheError <- errors.New("add notification timeout")
		}

	}

}

// Watch pulls stuff from etcd, converts, and pushes out the outgoing channel. Meant to be
// called as a goroutine.
func (w *userProjectWatcher) Watch() {
	defer close(w.outgoing)
	defer func() {
		// when the watch ends, always remove the watcher from the cache to avoid leaking.
		w.authCache.RemoveWatcher(w)
	}()
	defer utilruntime.HandleCrash()

	// start by emitting all the `initialProjects`
	for i := range w.initialProjects {
		// keep this check here to sure we don't keep this open in the case of failures
		select {
		case err := <-w.cacheError:
			w.emit(makeErrorEvent(err))
			return
		default:
		}

		w.emit(watch.Event{
			Type:   watch.Added,
			Object: projectutil.ConvertNamespace(&w.initialProjects[i]),
		})
	}

	for {
		select {
		case err := <-w.cacheError:
			w.emit(makeErrorEvent(err))
			return

		case <-w.userStop:
			return

		case event := <-w.cacheIncoming:
			if curLen := int64(len(w.cacheIncoming)); watchChannelHWM.Update(curLen) {
				// Monitor if this gets backed up, and how much.
				glog.V(2).Infof("watch: %v objects queued in project cache watching channel.", curLen)
			}

			w.emit(event)
		}
	}
}

func makeErrorEvent(err error) watch.Event {
	return watch.Event{
		Type: watch.Error,
		Object: &metav1.Status{
			Status:  metav1.StatusFailure,
			Message: err.Error(),
		},
	}
}

// ResultChan implements watch.Interface.
func (w *userProjectWatcher) ResultChan() <-chan watch.Event {
	return w.outgoing
}

// Stop implements watch.Interface.
func (w *userProjectWatcher) Stop() {
	// lock access so we don't race past the channel select
	w.stopLock.Lock()
	defer w.stopLock.Unlock()

	// Prevent double channel closes.
	select {
	case <-w.userStop:
		return
	default:
	}
	close(w.userStop)
}
