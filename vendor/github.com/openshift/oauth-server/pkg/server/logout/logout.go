package logout

import (
	"net/http"

	"github.com/RangelReale/osin"
	"k8s.io/klog"

	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/openshift/oauth-server/pkg"
	"github.com/openshift/oauth-server/pkg/server/redirect"
	"github.com/openshift/oauth-server/pkg/server/session"
)

const thenParam = "then"

func NewLogout(invalidator session.SessionInvalidator, redirect string) oauthserver.Endpoints {
	return &logout{
		invalidator: invalidator,
		redirect:    redirect,
	}
}

type logout struct {
	invalidator session.SessionInvalidator
	redirect    string
}

func (l *logout) Install(mux oauthserver.Mux, prefix string) {
	mux.Handle(prefix, l)
}

func (l *logout) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// TODO while having a POST provides some protection, this endpoint is invokable via JS.
	// we could easily add CSRF protection, but then it would make it really hard for the console
	// to actually use this endpoint.  we could have some alternative logout path that validates
	// the request based on the OAuth client secret, but all of that seems overkill for logout.
	// to make this perfectly safe, we would need the console to redirect to this page and then
	// have the user click logout.  forgo that for now to keep the UX of kube:admin clean.
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// invalidate with empty user to force session removal
	if err := l.invalidator.InvalidateAuthentication(w, &user.DefaultInfo{}); err != nil {
		klog.V(5).Infof("error logging out: %v", err)
		http.Error(w, "failed to log out", http.StatusInternalServerError)
		return
	}

	// optionally redirect if safe to do so
	if then := req.FormValue(thenParam); l.isValidRedirect(then) {
		http.Redirect(w, req, then, http.StatusFound)
		return
	}
}

func (l *logout) isValidRedirect(then string) bool {
	if redirect.IsServerRelativeURL(then) {
		return true
	}

	return osin.ValidateUri(l.redirect, then) == nil
}
