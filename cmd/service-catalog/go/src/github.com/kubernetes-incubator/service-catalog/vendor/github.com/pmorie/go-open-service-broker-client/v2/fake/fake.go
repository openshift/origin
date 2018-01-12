package fake

import (
	"errors"
	"net/http"
	"sync"

	"github.com/pmorie/go-open-service-broker-client/v2"
)

// NewFakeClientFunc returns a v2.CreateFunc that returns a FakeClient with
// the given FakeClientConfiguration.  It is useful for injecting the
// FakeClient in code that uses the v2.CreateFunc interface.
func NewFakeClientFunc(config FakeClientConfiguration) v2.CreateFunc {
	return func(_ *v2.ClientConfiguration) (v2.Client, error) {
		return NewFakeClient(config), nil
	}
}

// ReturnFakeClientFunc returns a v2.CreateFunc that returns the given
// FakeClient.
func ReturnFakeClientFunc(c *FakeClient) v2.CreateFunc {
	return func(_ *v2.ClientConfiguration) (v2.Client, error) {
		return c, nil
	}
}

// NewFakeClient returns a new fake Client with the given
// FakeClientConfiguration.
func NewFakeClient(config FakeClientConfiguration) *FakeClient {
	return &FakeClient{
		CatalogReaction:                  config.CatalogReaction,
		ProvisionReaction:                config.ProvisionReaction,
		UpdateInstanceReaction:           config.UpdateInstanceReaction,
		DeprovisionReaction:              config.DeprovisionReaction,
		PollLastOperationReaction:        config.PollLastOperationReaction,
		PollLastOperationReactions:       config.PollLastOperationReactions,
		PollBindingLastOperationReaction: config.PollBindingLastOperationReaction,
		BindReaction:                     config.BindReaction,
		UnbindReaction:                   config.UnbindReaction,
		GetBindingReaction:               config.GetBindingReaction,
	}
}

// FakeClientConfiguration models the configuration of a FakeClient.
type FakeClientConfiguration struct {
	CatalogReaction                  CatalogReactionInterface
	ProvisionReaction                ProvisionReactionInterface
	UpdateInstanceReaction           UpdateInstanceReactionInterface
	DeprovisionReaction              DeprovisionReactionInterface
	PollLastOperationReaction        PollLastOperationReactionInterface
	PollLastOperationReactions       map[v2.OperationKey]*PollLastOperationReaction
	PollBindingLastOperationReaction PollBindingLastOperationReactionInterface
	BindReaction                     BindReactionInterface
	UnbindReaction                   UnbindReactionInterface
	GetBindingReaction               GetBindingReactionInterface
}

// Action is a record of a method call on the FakeClient.
type Action struct {
	Type    ActionType
	Request interface{}
}

// ActionType is a typedef over the set of actions that can be taken on a
// FakeClient.
type ActionType string

// These are the set of actions that can be taken on a FakeClient.
const (
	GetCatalog               ActionType = "GetCatalog"
	ProvisionInstance        ActionType = "ProvisionInstance"
	UpdateInstance           ActionType = "UpdateInstance"
	DeprovisionInstance      ActionType = "DeprovisionInstance"
	PollLastOperation        ActionType = "PollLastOperation"
	PollBindingLastOperation ActionType = "PollBindingLastOperation"
	Bind                     ActionType = "Bind"
	Unbind                   ActionType = "Unbind"
	GetBinding               ActionType = "GetBinding"
)

// FakeClient is a fake implementation of the v2.Client interface. It records
// the actions that are taken on it and runs the appropriate reaction to those
// actions. If an action for which there is no reaction specified occurs, it
// returns an error.  FakeClient is threadsafe.
type FakeClient struct {
	CatalogReaction                  CatalogReactionInterface
	ProvisionReaction                ProvisionReactionInterface
	UpdateInstanceReaction           UpdateInstanceReactionInterface
	DeprovisionReaction              DeprovisionReactionInterface
	PollLastOperationReaction        PollLastOperationReactionInterface
	PollLastOperationReactions       map[v2.OperationKey]*PollLastOperationReaction
	PollBindingLastOperationReaction PollBindingLastOperationReactionInterface
	BindReaction                     BindReactionInterface
	UnbindReaction                   UnbindReactionInterface
	GetBindingReaction               GetBindingReactionInterface

	sync.Mutex
	actions []Action
}

var _ v2.Client = &FakeClient{}

// Actions is a method defined on FakeClient that returns the actions taken on
// it.
func (c *FakeClient) Actions() []Action {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	return c.actions
}

// GetCatalog implements the Client.GetCatalog method for the FakeClient.
func (c *FakeClient) GetCatalog() (*v2.CatalogResponse, error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	c.actions = append(c.actions, Action{Type: GetCatalog})

	if c.CatalogReaction != nil {
		return c.CatalogReaction.react()
	}

	return nil, UnexpectedActionError()
}

// ProvisionInstance implements the Client.ProvisionRequest method for the
// FakeClient.
func (c *FakeClient) ProvisionInstance(r *v2.ProvisionRequest) (*v2.ProvisionResponse, error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	c.actions = append(c.actions, Action{ProvisionInstance, r})

	if c.ProvisionReaction != nil {
		return c.ProvisionReaction.react(r)
	}

	return nil, UnexpectedActionError()
}

// UpdateInstance implements the Client.UpdateInstance method for the
// FakeClient.
func (c *FakeClient) UpdateInstance(r *v2.UpdateInstanceRequest) (*v2.UpdateInstanceResponse, error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	c.actions = append(c.actions, Action{UpdateInstance, r})

	if c.UpdateInstanceReaction != nil {
		return c.UpdateInstanceReaction.react(r)
	}

	return nil, UnexpectedActionError()
}

// DeprovisionInstance implements the Client.DeprovisionInstance method on the
// FakeClient.
func (c *FakeClient) DeprovisionInstance(r *v2.DeprovisionRequest) (*v2.DeprovisionResponse, error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	c.actions = append(c.actions, Action{DeprovisionInstance, r})

	if c.DeprovisionReaction != nil {
		return c.DeprovisionReaction.react(r)
	}

	return nil, UnexpectedActionError()
}

// PollLastOperation implements the Client.PollLastOperation method on the
// FakeClient.
func (c *FakeClient) PollLastOperation(r *v2.LastOperationRequest) (*v2.LastOperationResponse, error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	c.actions = append(c.actions, Action{PollLastOperation, r})

	if r.OperationKey != nil && c.PollLastOperationReactions[*r.OperationKey] != nil {
		return c.PollLastOperationReactions[*r.OperationKey].Response, c.PollLastOperationReactions[*r.OperationKey].Error
	} else if c.PollLastOperationReaction != nil {
		return c.PollLastOperationReaction.react(r)
	}

	return nil, UnexpectedActionError()
}

// PollBindingLastOperation implements the Client.PollBindingLastOperation
// method on the FakeClient.
func (c *FakeClient) PollBindingLastOperation(r *v2.BindingLastOperationRequest) (*v2.LastOperationResponse, error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	c.actions = append(c.actions, Action{PollBindingLastOperation, r})

	if c.PollBindingLastOperationReaction != nil {
		return c.PollBindingLastOperationReaction.react(r)
	}

	return nil, UnexpectedActionError()
}

// Bind implements the Client.Bind method on the FakeClient.
func (c *FakeClient) Bind(r *v2.BindRequest) (*v2.BindResponse, error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	c.actions = append(c.actions, Action{Bind, r})

	if c.BindReaction != nil {
		return c.BindReaction.react(r)
	}

	return nil, UnexpectedActionError()
}

// Unbind implements the Client.Unbind method on the FakeClient.
func (c *FakeClient) Unbind(r *v2.UnbindRequest) (*v2.UnbindResponse, error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	c.actions = append(c.actions, Action{Unbind, r})

	if c.UnbindReaction != nil {
		return c.UnbindReaction.react(r)
	}

	return nil, UnexpectedActionError()
}

// GetBinding implements the Client.GetBinding method for the FakeClient.
func (c *FakeClient) GetBinding(*v2.GetBindingRequest) (*v2.GetBindingResponse, error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	c.actions = append(c.actions, Action{Type: GetBinding})

	if c.GetBindingReaction != nil {
		return c.GetBindingReaction.react()
	}

	return nil, UnexpectedActionError()
}

// UnexpectedActionError returns an error message when an action is not found
// in the FakeClient's action array.
func UnexpectedActionError() error {
	return errors.New("Unexpected action")
}

// CatalogReactionInterface defines the reaction to GetCatalog requests.
type CatalogReactionInterface interface {
	react() (*v2.CatalogResponse, error)
}

type CatalogReaction struct {
	Response *v2.CatalogResponse
	Error    error
}

func (r *CatalogReaction) react() (*v2.CatalogResponse, error) {
	if r == nil {
		return nil, UnexpectedActionError()
	}
	return r.Response, r.Error
}

type DynamicCatalogReaction func() (*v2.CatalogResponse, error)

func (r DynamicCatalogReaction) react() (*v2.CatalogResponse, error) {
	return r()
}

// ProvisionReactionInterface defines the reaction to ProvisionInstance requests.
type ProvisionReactionInterface interface {
	react(*v2.ProvisionRequest) (*v2.ProvisionResponse, error)
}

type ProvisionReaction struct {
	Response *v2.ProvisionResponse
	Error    error
}

func (r *ProvisionReaction) react(_ *v2.ProvisionRequest) (*v2.ProvisionResponse, error) {
	if r == nil {
		return nil, UnexpectedActionError()
	}
	return r.Response, r.Error
}

type DynamicProvisionReaction func(*v2.ProvisionRequest) (*v2.ProvisionResponse, error)

func (r DynamicProvisionReaction) react(req *v2.ProvisionRequest) (*v2.ProvisionResponse, error) {
	return r(req)
}

// UpdateInstanceReactionInterface defines the reaction to UpdateInstance requests.
type UpdateInstanceReactionInterface interface {
	react(*v2.UpdateInstanceRequest) (*v2.UpdateInstanceResponse, error)
}

type UpdateInstanceReaction struct {
	Response *v2.UpdateInstanceResponse
	Error    error
}

func (r *UpdateInstanceReaction) react(_ *v2.UpdateInstanceRequest) (*v2.UpdateInstanceResponse, error) {
	if r == nil {
		return nil, UnexpectedActionError()
	}
	return r.Response, r.Error
}

type DynamicUpdateInstanceReaction func(*v2.UpdateInstanceRequest) (*v2.UpdateInstanceResponse, error)

func (r DynamicUpdateInstanceReaction) react(req *v2.UpdateInstanceRequest) (*v2.UpdateInstanceResponse, error) {
	return r(req)
}

// DeprovisionReactionInterface defines the reaction to DeprovisionInstance requests.
type DeprovisionReactionInterface interface {
	react(*v2.DeprovisionRequest) (*v2.DeprovisionResponse, error)
}

type DeprovisionReaction struct {
	Response *v2.DeprovisionResponse
	Error    error
}

func (r *DeprovisionReaction) react(_ *v2.DeprovisionRequest) (*v2.DeprovisionResponse, error) {
	if r == nil {
		return nil, UnexpectedActionError()
	}
	return r.Response, r.Error
}

type DynamicDeprovisionReaction func(*v2.DeprovisionRequest) (*v2.DeprovisionResponse, error)

func (r DynamicDeprovisionReaction) react(req *v2.DeprovisionRequest) (*v2.DeprovisionResponse, error) {
	return r(req)
}

// PollLastOperationReactionInterface defines the reaction to PollLastOperation
// requests.
type PollLastOperationReactionInterface interface {
	react(*v2.LastOperationRequest) (*v2.LastOperationResponse, error)
}

type PollLastOperationReaction struct {
	Response *v2.LastOperationResponse
	Error    error
}

func (r *PollLastOperationReaction) react(_ *v2.LastOperationRequest) (*v2.LastOperationResponse, error) {
	if r == nil {
		return nil, UnexpectedActionError()
	}
	return r.Response, r.Error
}

type DynamicPollLastOperationReaction func(*v2.LastOperationRequest) (*v2.LastOperationResponse, error)

func (r DynamicPollLastOperationReaction) react(req *v2.LastOperationRequest) (*v2.LastOperationResponse, error) {
	return r(req)
}

// PollBindingLastOperationReactionInterface defines the reaction to PollLastOperation
// requests.
type PollBindingLastOperationReactionInterface interface {
	react(*v2.BindingLastOperationRequest) (*v2.LastOperationResponse, error)
}

type PollBindingLastOperationReaction struct {
	Response *v2.LastOperationResponse
	Error    error
}

func (r *PollBindingLastOperationReaction) react(_ *v2.BindingLastOperationRequest) (*v2.LastOperationResponse, error) {
	if r == nil {
		return nil, UnexpectedActionError()
	}
	return r.Response, r.Error
}

type DynamicPollBindingLastOperationReaction func(*v2.BindingLastOperationRequest) (*v2.LastOperationResponse, error)

func (r DynamicPollBindingLastOperationReaction) react(req *v2.BindingLastOperationRequest) (*v2.LastOperationResponse, error) {
	return r(req)
}

// BindReactionInterface defines the reaction to Bind requests.
type BindReactionInterface interface {
	react(*v2.BindRequest) (*v2.BindResponse, error)
}

type BindReaction struct {
	Response *v2.BindResponse
	Error    error
}

func (r *BindReaction) react(_ *v2.BindRequest) (*v2.BindResponse, error) {
	if r == nil {
		return nil, UnexpectedActionError()
	}
	return r.Response, r.Error
}

type DynamicBindReaction func(*v2.BindRequest) (*v2.BindResponse, error)

func (r DynamicBindReaction) react(req *v2.BindRequest) (*v2.BindResponse, error) {
	return r(req)
}

// UnbindReactionInterface defines the reaction to Unbind requests.
type UnbindReactionInterface interface {
	react(*v2.UnbindRequest) (*v2.UnbindResponse, error)
}

type UnbindReaction struct {
	Response *v2.UnbindResponse
	Error    error
}

func (r *UnbindReaction) react(_ *v2.UnbindRequest) (*v2.UnbindResponse, error) {
	if r == nil {
		return nil, UnexpectedActionError()
	}
	return r.Response, r.Error
}

type DynamicUnbindReaction func(*v2.UnbindRequest) (*v2.UnbindResponse, error)

func (r DynamicUnbindReaction) react(req *v2.UnbindRequest) (*v2.UnbindResponse, error) {
	return r(req)
}

// GetBindingReactionInterface defines the reaction to GetBinding requests.
type GetBindingReactionInterface interface {
	react() (*v2.GetBindingResponse, error)
}

type GetBindingReaction struct {
	Response *v2.GetBindingResponse
	Error    error
}

func (r *GetBindingReaction) react() (*v2.GetBindingResponse, error) {
	if r == nil {
		return nil, UnexpectedActionError()
	}
	return r.Response, r.Error
}

type DynamicGetBindingReaction func() (*v2.GetBindingResponse, error)

func (r DynamicGetBindingReaction) react() (*v2.GetBindingResponse, error) {
	return r()
}

func strPtr(s string) *string {
	return &s
}

// AsyncRequiredError returns error for required asynchronous operations.
func AsyncRequiredError() error {
	return v2.HTTPStatusCodeError{
		StatusCode:   http.StatusUnprocessableEntity,
		ErrorMessage: strPtr(v2.AsyncErrorMessage),
		Description:  strPtr(v2.AsyncErrorDescription),
	}
}

// AppGUIDRequiredError returns error for when app GUID is missing from bind
// request.
func AppGUIDRequiredError() error {
	return v2.HTTPStatusCodeError{
		StatusCode:   http.StatusUnprocessableEntity,
		ErrorMessage: strPtr(v2.AppGUIDRequiredErrorMessage),
		Description:  strPtr(v2.AppGUIDRequiredErrorDescription),
	}
}
