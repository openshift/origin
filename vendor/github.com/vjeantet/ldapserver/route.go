package ldapserver

import "reflect"

// Constant to LDAP Request protocol Type names
const (
	SEARCH   = "SearchRequest"
	BIND     = "BindRequest"
	COMPARE  = "CompareRequest"
	ADD      = "AddRequest"
	MODIFY   = "ModifyRequest"
	DELETE   = "DeleteRequest"
	EXTENDED = "ExtendedRequest"
	ABANDON  = "AbandonRequest"
)

// HandlerFunc type is an adapter to allow the use of
// ordinary functions as LDAP handlers.  If f is a function
// with the appropriate signature, HandlerFunc(f) is a
// Handler object that calls f.
type HandlerFunc func(ResponseWriter, *Message)

// RouteMux manages all routes
type RouteMux struct {
	routes        []*route
	notFoundRoute *route
}

type route struct {
	operation string
	handler   HandlerFunc
	exoName   LDAPOID
	sBasedn   string
	sFilter   string
}

// Match return true when the *Message matches the route
// conditions
func (r *route) Match(m *Message) bool {
	if reflect.TypeOf(m.protocolOp).Name() != r.operation {
		return false
	}

	switch v := m.protocolOp.(type) {
	case ExtendedRequest:
		if "" != r.exoName {
			if v.GetResponseName() == r.exoName {
				return true
			}
			return false
		}

	case SearchRequest:
		if "" != r.sBasedn {
			if string(v.GetBaseObject()) == r.sBasedn {
				return true
			}
			return false
		}
	}
	return true
}

func (r *route) BaseDn(dn string) *route {
	r.sBasedn = dn
	return r
}

func (r *route) Filter(pattern string) *route {
	r.sFilter = pattern
	return r
}

func (r *route) RequestName(name LDAPOID) *route {
	r.exoName = name
	return r
}

// NewRouteMux returns a new *RouteMux
// RouteMux implements ldapserver.Handler
func NewRouteMux() *RouteMux {
	return &RouteMux{}
}

// Handler interface used to serve a LDAP Request message
type Handler interface {
	ServeLDAP(w ResponseWriter, r *Message)
}

// ServeLDAP dispatches the request to the handler whose
// pattern most closely matches the request request Message.
func (h *RouteMux) ServeLDAP(w ResponseWriter, r *Message) {

	//find a matching Route
	for _, route := range h.routes {

		//if the route don't match, skip it
		if route.Match(r) == false {
			continue
		}

		route.handler(w, r)
		return
	}

	if h.notFoundRoute != nil {
		h.notFoundRoute.handler(w, r)
	} else {
		res := NewResponse(LDAPResultUnwillingToPerform)
		res.DiagnosticMessage = "Operation not implemented by server"
		w.Write(res)
	}
}

// Adds a new Route to the Handler
func (h *RouteMux) addRoute(r *route) {
	//and finally append to the list of Routes
	//create the Route
	h.routes = append(h.routes, r)
}

func (h *RouteMux) NotFound(handler HandlerFunc) {
	route := &route{}
	route.handler = handler
	h.notFoundRoute = route
}

func (h *RouteMux) Bind(handler HandlerFunc) *route {
	route := &route{}
	route.operation = BIND
	route.handler = handler
	h.addRoute(route)
	return route
}

func (h *RouteMux) Search(handler HandlerFunc) *route {
	route := &route{}
	route.operation = SEARCH
	route.handler = handler
	h.addRoute(route)
	return route
}

func (h *RouteMux) Add(handler HandlerFunc) *route {
	route := &route{}
	route.operation = ADD
	route.handler = handler
	h.addRoute(route)
	return route
}

func (h *RouteMux) Delete(handler HandlerFunc) *route {
	route := &route{}
	route.operation = DELETE
	route.handler = handler
	h.addRoute(route)
	return route
}

func (h *RouteMux) Modify(handler HandlerFunc) *route {
	route := &route{}
	route.operation = MODIFY
	route.handler = handler
	h.addRoute(route)
	return route
}

func (h *RouteMux) Compare(handler HandlerFunc) *route {
	route := &route{}
	route.operation = COMPARE
	route.handler = handler
	h.addRoute(route)
	return route
}

func (h *RouteMux) Extended(handler HandlerFunc) *route {
	route := &route{}
	route.operation = EXTENDED
	route.handler = handler
	h.addRoute(route)
	return route
}

func (h *RouteMux) Abandon(handler HandlerFunc) *route {
	route := &route{}
	route.operation = ABANDON
	route.handler = handler
	h.addRoute(route)
	return route
}
