package logout

import (
	"net/http"

	"github.com/golang/glog"

	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/openshift/origin/pkg/oauthserver"
	"github.com/openshift/origin/pkg/oauthserver/server/headers"
	"github.com/openshift/origin/pkg/oauthserver/server/redirect"
	"github.com/openshift/origin/pkg/oauthserver/server/session"
	"github.com/openshift/origin/pkg/oauthserver/server/tokenrequest"
)

const thenParam = "then"

func NewLogout(invalidator session.SessionInvalidator) tokenrequest.Endpoints {
	return &logout{
		invalidator: invalidator,
	}
}

type logout struct {
	invalidator session.SessionInvalidator
}

func (l *logout) Install(mux oauthserver.Mux, paths ...string) {
	for _, path := range paths {
		mux.Handle(path, l)
	}
}

func (l *logout) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// TODO this seems like something that should happen automatically at a higher level
	// we also do not set these headers on the OAuth endpoints or the token request endpoint...
	headers.SetStandardHeaders(w)

	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// invalidate with empty user to force session removal
	if err := l.invalidator.InvalidateAuthentication(w, &user.DefaultInfo{}); err != nil {
		glog.V(5).Infof("error logging out: %v", err)
		http.Error(w, "failed to log out", http.StatusInternalServerError)
		return
	}

	if then := req.FormValue(thenParam); redirect.IsServerRelativeURL(then) {
		http.Redirect(w, req, then, http.StatusFound)
		return
	}
}
