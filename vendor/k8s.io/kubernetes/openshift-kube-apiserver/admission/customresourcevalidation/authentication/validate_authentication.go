package authentication

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"math"
	"slices"
	"time"

	"golang.org/x/sync/singleflight"
	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/library"
	"k8s.io/apiserver/pkg/warning"
	"k8s.io/klog/v2"
	"k8s.io/utils/lru"

	"github.com/google/cel-go/checker"

	configv1 "github.com/openshift/api/config/v1"
	authenticationcel "k8s.io/apiserver/pkg/authentication/cel"
	crvalidation "k8s.io/kubernetes/openshift-kube-apiserver/admission/customresourcevalidation"
)

const PluginName = "config.openshift.io/ValidateAuthentication"

const (
	wholeResourceExcessiveCostThreshold = 100000000
	excessiveCompileDuration            = time.Second
	costlyExpressionWarningCount        = 3

	// This is the default KAS request header size limit in bytes.
	// Because JWTs are only limited in size by the maximum request header size,
	// we can use this fixed value to make pessimistic size estimates by assuming
	// that the inputs were decoded from base64-encoded JSON.
	//
	// This isn't very precise, but can still be used to provide
	// end-users a signal that they are potentially doing very expensive
	// operations with CEL expressions whose cost is dependent
	// on the size of the input.
	fixedSize = 1 << 20
)

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return crvalidation.NewValidator(
			map[schema.GroupResource]bool{
				configv1.GroupVersion.WithResource("authentications").GroupResource(): true,
			},
			map[schema.GroupVersionKind]crvalidation.ObjectValidator{
				configv1.GroupVersion.WithKind("Authentication"): authenticationV1{
					cel: defaultCelStore(),
				},
			})
	})
}

func toAuthenticationV1(uncastObj runtime.Object) (*configv1.Authentication, field.ErrorList) {
	if uncastObj == nil {
		return nil, nil
	}

	obj, ok := uncastObj.(*configv1.Authentication)
	if !ok {
		return nil, field.ErrorList{
			field.NotSupported(field.NewPath("kind"), fmt.Sprintf("%T", uncastObj), []string{"Authentication"}),
			field.NotSupported(field.NewPath("apiVersion"), fmt.Sprintf("%T", uncastObj), []string{"config.openshift.io/v1"}),
		}
	}

	return obj, nil
}

type celStore struct {
	compilingGroup singleFlightDoer
	compiledStore  compiledExpressionStore
	compiler       authenticationcel.Compiler
	sizeEstimator  checker.CostEstimator
	timerFactory   timerFactory
}

func defaultCelStore() *celStore {
	return &celStore{
		compiledStore:  lru.New(100),
		compilingGroup: new(singleflight.Group),
		compiler:       authenticationcel.NewDefaultCompiler(),
		sizeEstimator: &fixedSizeEstimator{
			size: fixedSize,
		},
		timerFactory: &excessiveCompileTimerFactory{},
	}
}

type singleFlightDoer interface {
	Do(key string, fn func() (any, error)) (any, error, bool)
}

type compiledExpressionStore interface {
	Add(key lru.Key, value interface{})
	Get(key lru.Key) (value interface{}, ok bool)
}

type timerFactory interface {
	Timer(time.Duration, func()) timer
}

type timer interface {
	Stop() bool
}

type excessiveCompileTimerFactory struct{}

func (ectf *excessiveCompileTimerFactory) Timer(duration time.Duration, do func()) timer {
	return time.AfterFunc(duration, do)
}

type authenticationV1 struct {
	cel *celStore
}

func (a authenticationV1) ValidateCreate(ctx context.Context, uncastObj runtime.Object) field.ErrorList {
	obj, errs := toAuthenticationV1(uncastObj)
	if len(errs) > 0 {
		return errs
	}

	errs = append(errs, validation.ValidateObjectMeta(&obj.ObjectMeta, false, crvalidation.RequireNameCluster, field.NewPath("metadata"))...)
	errs = append(errs, validateAuthenticationSpecCreate(ctx, obj.Spec, a.cel)...)

	return errs
}

func (a authenticationV1) ValidateUpdate(ctx context.Context, uncastObj runtime.Object, uncastOldObj runtime.Object) field.ErrorList {
	obj, errs := toAuthenticationV1(uncastObj)
	if len(errs) > 0 {
		return errs
	}
	oldObj, errs := toAuthenticationV1(uncastOldObj)
	if len(errs) > 0 {
		return errs
	}

	errs = append(errs, validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &oldObj.ObjectMeta, field.NewPath("metadata"))...)
	errs = append(errs, validateAuthenticationSpecUpdate(ctx, obj.Spec, oldObj.Spec, a.cel)...)

	return errs
}

func (authenticationV1) ValidateStatusUpdate(_ context.Context, uncastObj runtime.Object, uncastOldObj runtime.Object) field.ErrorList {
	obj, errs := toAuthenticationV1(uncastObj)
	if len(errs) > 0 {
		return errs
	}
	oldObj, errs := toAuthenticationV1(uncastOldObj)
	if len(errs) > 0 {
		return errs
	}

	errs = append(errs, validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &oldObj.ObjectMeta, field.NewPath("metadata"))...)
	errs = append(errs, validateAuthenticationStatus(obj.Status)...)

	return errs
}

func validateAuthenticationSpecCreate(ctx context.Context, spec configv1.AuthenticationSpec, cel *celStore) field.ErrorList {
	return validateAuthenticationSpec(ctx, spec, cel)
}

func validateAuthenticationSpecUpdate(ctx context.Context, newspec, oldspec configv1.AuthenticationSpec, cel *celStore) field.ErrorList {
	return validateAuthenticationSpec(ctx, newspec, cel)
}

func validateAuthenticationSpec(ctx context.Context, spec configv1.AuthenticationSpec, cel *celStore) field.ErrorList {
	errs := field.ErrorList{}
	specField := field.NewPath("spec")

	if spec.WebhookTokenAuthenticator != nil {
		switch spec.Type {
		case configv1.AuthenticationTypeNone, configv1.AuthenticationTypeIntegratedOAuth, "":
			// validate the secret name in WebhookTokenAuthenticator
			errs = append(
				errs,
				crvalidation.ValidateSecretReference(
					specField.Child("webhookTokenAuthenticator").Child("kubeConfig"),
					spec.WebhookTokenAuthenticator.KubeConfig,
					false,
				)...,
			)
		default:
			errs = append(errs, field.Invalid(specField.Child("webhookTokenAuthenticator"),
				spec.WebhookTokenAuthenticator, fmt.Sprintf("this field cannot be set with the %q .spec.type", spec.Type),
			))
		}
	}

	errs = append(errs, crvalidation.ValidateConfigMapReference(specField.Child("oauthMetadata"), spec.OAuthMetadata, false)...)

	// Perform External OIDC Provider related validations
	// ----------------

	// There is currently no guarantee that these fields are not set when the spec.Type is != OIDC.
	// To ensure we are enforcing approriate admission validations at all times, just always iterate through the list
	// of OIDC Providers and perform the validations.
	// If/when the openshift/api admission validations are updated to enforce that this field is not configured
	// when Type != OIDC, this loop should be a no-op due to an empty list.
	for i, provider := range spec.OIDCProviders {
		errs = append(errs, validateOIDCProvider(ctx, specField.Child("oidcProviders").Index(i), cel, provider)...)
	}
	// ----------------

	return errs
}

func validateAuthenticationStatus(status configv1.AuthenticationStatus) field.ErrorList {
	return crvalidation.ValidateConfigMapReference(field.NewPath("status", "integratedOAuthMetadata"), status.IntegratedOAuthMetadata, false)
}

type costRecorder struct {
	Recordings []costRecording
}

func (cr *costRecorder) AddRecording(field *field.Path, cost uint64) {
	cr.Recordings = append(cr.Recordings, costRecording{
		Field: field,
		Cost:  cost,
	})
}

type costRecording struct {
	Field *field.Path
	Cost  uint64
}

func validateOIDCProvider(ctx context.Context, path *field.Path, cel *celStore, provider configv1.OIDCProvider) field.ErrorList {
	costRecorder := &costRecorder{}

	errs := validateClaimMappings(ctx, path, cel, costRecorder, provider.ClaimMappings)

	var totalCELExpressionCost uint64 = 0

	for _, recording := range costRecorder.Recordings {
		totalCELExpressionCost = addCost(totalCELExpressionCost, recording.Cost)
	}

	if totalCELExpressionCost > wholeResourceExcessiveCostThreshold {
		costlyExpressions := getNMostCostlyExpressions(costlyExpressionWarningCount, costRecorder.Recordings...)
		warn := fmt.Sprintf("runtime cost of all CEL expressions exceeds %d points. top %d most costly expressions: %v", wholeResourceExcessiveCostThreshold, len(costlyExpressions), costlyExpressions)
		warning.AddWarning(ctx, "", warn)
		klog.Warning(warn)
	}

	return errs
}

// addCost adds a cost value to a total value,
// returning the resulting value.
// addCost handles integer overflow errors
// by just always returning the maximum uint64
// value if an overflow would occur.
func addCost(total, cost uint64) uint64 {
	if total > math.MaxUint64-cost {
		return math.MaxUint64
	}

	return total + cost
}

func getNMostCostlyExpressions(n int, records ...costRecording) []costRecording {
	// sort in descending order of cost
	slices.SortFunc(records, func(a, b costRecording) int {
		return cmp.Compare(b.Cost, a.Cost)
	})

	// safely get the N most expensive cost records
	if len(records) > n {
		return records[:n]
	}

	return records
}

func validateClaimMappings(ctx context.Context, path *field.Path, cel *celStore, costRecorder *costRecorder, claimMappings configv1.TokenClaimMappings) field.ErrorList {
	path = path.Child("claimMappings")

	out := field.ErrorList{}

	out = append(out, validateUIDClaimMapping(ctx, path, cel, costRecorder, claimMappings.UID)...)
	out = append(out, validateExtraClaimMapping(ctx, path, cel, costRecorder, claimMappings.Extra...)...)

	return out
}

func validateUIDClaimMapping(ctx context.Context, path *field.Path, cel *celStore, costRecorder *costRecorder, uid *configv1.TokenClaimOrExpressionMapping) field.ErrorList {
	if uid == nil {
		return nil
	}

	out := field.ErrorList{}

	if uid.Expression != "" {
		childPath := path.Child("uid", "expression")

		out = append(out, validateCELExpression(ctx, cel, costRecorder, childPath, &authenticationcel.ClaimMappingExpression{
			Expression: uid.Expression,
		})...)
	}

	return out
}

func validateExtraClaimMapping(ctx context.Context, path *field.Path, cel *celStore, costRecorder *costRecorder, extras ...configv1.ExtraMapping) field.ErrorList {
	out := field.ErrorList{}
	for i, extra := range extras {
		out = append(out, validateExtra(ctx, path.Child("extra").Index(i), cel, costRecorder, extra)...)
	}

	return out
}

func validateExtra(ctx context.Context, path *field.Path, cel *celStore, costRecorder *costRecorder, extra configv1.ExtraMapping) field.ErrorList {
	childPath := path.Child("valueExpression")

	return validateCELExpression(ctx, cel, costRecorder, childPath, &authenticationcel.ExtraMappingExpression{
		Key:        extra.Key,
		Expression: extra.ValueExpression,
	})
}

type celCompileResult struct {
	err  error
	cost uint64
}

func validateCELExpression(ctx context.Context, cel *celStore, costRecorder *costRecorder, path *field.Path, accessor authenticationcel.ExpressionAccessor) field.ErrorList {
	// if context has been canceled, don't try to compile any expressions
	if err := ctx.Err(); err != nil {
		return field.ErrorList{field.InternalError(path, err)}
	}

	result, err, _ := cel.compilingGroup.Do(accessor.GetExpression(), func() (interface{}, error) {
		// if the expression is not currently being compiled, it might have already been compiled
		if val, ok := cel.compiledStore.Get(accessor.GetExpression()); ok {
			res, ok := val.(celCompileResult)
			if !ok {
				return nil, fmt.Errorf("expected return value from cache of compiled expressions to be of type celCompileResult but was %T", val)
			}

			return res, nil
		}

		// expression is not currently being compiled, and has not been compiled before (or has been long enough since it was last compiled that we dropped it).
		// Let's compile it.

		// Asynchronously handle excessive compilation time so we
		// can still log a warning in the event the process has died
		// before compilation of the expression has finished.
		warningChan := make(chan string, 1)
		timer := cel.timerFactory.Timer(excessiveCompileDuration, func() {
			defer close(warningChan)
			warn := fmt.Sprintf("cel expression %q took excessively long to compile (%s)", accessor.GetExpression(), excessiveCompileDuration)
			klog.Warning(warn)
			warningChan <- warn
		})

		compRes, compErr := cel.compiler.CompileClaimsExpression(accessor)

		timer.Stop()

		res := celCompileResult{
			err: compErr,
		}

		if compRes.AST != nil && compErr == nil {
			cost, err := checker.Cost(compRes.AST.NativeRep(), &library.CostEstimator{
				SizeEstimator: cel.sizeEstimator,
			})
			// Because we are only warning on excessive cost, we shouldn't prevent the create/update of the resource if we can successfully
			// compile the expression but are unable to estimate the cost. The Structured Authentication Configuration feature does not
			// gate on cost of expressions, so we are doing a best-effort warning here.
			// Instead, default to our best estimate of the worst case cost.
			if err != nil {
				klog.Errorf("unable to estimate cost for expression %q: %v. Defaulting cost to %d", accessor.GetExpression(), err, fixedSize)
				cost = checker.CostEstimate{Max: fixedSize}
			}

			res.cost = cost.Max
		}

		// check if we received a warning related to excessive compile time. If not, continue
		select {
		case warn := <-warningChan:
			warning.AddWarning(ctx, "", warn)
		default:
		}

		cel.compiledStore.Add(accessor.GetExpression(), res)

		return res, nil
	})
	if err != nil {
		return field.ErrorList{field.InternalError(path, fmt.Errorf("running compilation of expression %q: %v", accessor.GetExpression(), err))}
	}

	compileRes, ok := result.(celCompileResult)
	if !ok {
		return field.ErrorList{field.InternalError(path, fmt.Errorf("expected result to be of type celCompileResult, but got %T", result))}
	}

	if compileRes.err != nil {
		return field.ErrorList{field.Invalid(path, accessor.GetExpression(), compileRes.err.Error())}
	}

	costRecorder.AddRecording(path, compileRes.cost)

	return nil
}

type fixedSizeEstimator struct {
	size uint64
}

func (fcse *fixedSizeEstimator) EstimateSize(element checker.AstNode) *checker.SizeEstimate {
	return &checker.SizeEstimate{Min: fcse.size, Max: fcse.size}
}

func (fcse *fixedSizeEstimator) EstimateCallCost(function, overloadID string, target *checker.AstNode, args []checker.AstNode) *checker.CallEstimate {
	return nil
}
