package apiservice

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"sort"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	kubetesting "k8s.io/client-go/testing"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	kubeaggregatorfake "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"

	operatorv1 "github.com/openshift/api/operator/v1"
	operatorlistersv1 "github.com/openshift/client-go/operator/listers/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

func TestDiffAPIServices(t *testing.T) {
	testCases := []struct {
		name              string
		oldAPIServices    []*apiregistrationv1.APIService
		newAPIServices    []*apiregistrationv1.APIService
		resultList        []string
		resultListChanged bool
	}{
		// scenario 1
		{
			name: "oauth removed",
			oldAPIServices: []*apiregistrationv1.APIService{
				newAPIService("authorization.openshift.io", "v1"),
				newAPIService("build.openshift.io", "v1"),
				newAPIService("image.openshift.io", "v1"),
				newAPIService("oauth.openshift.io", "v1"),
				newAPIService("route.openshift.io", "v1"),
				newAPIService("template.openshift.io", "v1"),
				newAPIService("user.openshift.io", "v1"),
			},
			newAPIServices: []*apiregistrationv1.APIService{
				newAPIService("authorization.openshift.io", "v1"),
				newAPIService("build.openshift.io", "v1"),
				newAPIService("image.openshift.io", "v1"),
				newAPIService("route.openshift.io", "v1"),
				newAPIService("template.openshift.io", "v1"),
				newAPIService("user.openshift.io", "v1"),
			},
			resultList: []string{
				"v1.authorization.openshift.io",
				"v1.build.openshift.io",
				"v1.image.openshift.io",
				"v1.route.openshift.io",
				"v1.template.openshift.io",
				"v1.user.openshift.io",
			},
			resultListChanged: true,
		},
		// scenario 2
		{
			name: "oauth added",
			oldAPIServices: []*apiregistrationv1.APIService{
				newAPIService("authorization.openshift.io", "v1"),
				newAPIService("build.openshift.io", "v1"),
				newAPIService("image.openshift.io", "v1"),
				newAPIService("route.openshift.io", "v1"),
				newAPIService("template.openshift.io", "v1"),
				newAPIService("user.openshift.io", "v1"),
			},
			newAPIServices: []*apiregistrationv1.APIService{
				newAPIService("authorization.openshift.io", "v1"),
				newAPIService("build.openshift.io", "v1"),
				newAPIService("image.openshift.io", "v1"),
				newAPIService("oauth.openshift.io", "v1"),
				newAPIService("route.openshift.io", "v1"),
				newAPIService("template.openshift.io", "v1"),
				newAPIService("user.openshift.io", "v1"),
			},
			resultList: []string{
				"v1.authorization.openshift.io",
				"v1.build.openshift.io",
				"v1.image.openshift.io",
				"v1.oauth.openshift.io",
				"v1.route.openshift.io",
				"v1.template.openshift.io",
				"v1.user.openshift.io",
			},
			resultListChanged: true,
		},
		// scenario 3
		{
			name: "oauth added, user removed",
			oldAPIServices: []*apiregistrationv1.APIService{
				newAPIService("authorization.openshift.io", "v1"),
				newAPIService("build.openshift.io", "v1"),
				newAPIService("image.openshift.io", "v1"),
				newAPIService("route.openshift.io", "v1"),
				newAPIService("template.openshift.io", "v1"),
				newAPIService("user.openshift.io", "v1"),
			},
			newAPIServices: []*apiregistrationv1.APIService{
				newAPIService("authorization.openshift.io", "v1"),
				newAPIService("build.openshift.io", "v1"),
				newAPIService("image.openshift.io", "v1"),
				newAPIService("oauth.openshift.io", "v1"),
				newAPIService("route.openshift.io", "v1"),
				newAPIService("template.openshift.io", "v1"),
			},
			resultList: []string{
				"v1.authorization.openshift.io",
				"v1.build.openshift.io",
				"v1.image.openshift.io",
				"v1.oauth.openshift.io",
				"v1.route.openshift.io",
				"v1.template.openshift.io",
			},
			resultListChanged: true,
		},
		// scenario 4
		{
			name: "no diff",
			oldAPIServices: []*apiregistrationv1.APIService{
				newAPIService("authorization.openshift.io", "v1"),
				newAPIService("build.openshift.io", "v1"),
				newAPIService("image.openshift.io", "v1"),
				newAPIService("oauth.openshift.io", "v1"),
				newAPIService("route.openshift.io", "v1"),
				newAPIService("template.openshift.io", "v1"),
				newAPIService("user.openshift.io", "v1"),
			},
			newAPIServices: []*apiregistrationv1.APIService{
				newAPIService("authorization.openshift.io", "v1"),
				newAPIService("build.openshift.io", "v1"),
				newAPIService("image.openshift.io", "v1"),
				newAPIService("oauth.openshift.io", "v1"),
				newAPIService("route.openshift.io", "v1"),
				newAPIService("template.openshift.io", "v1"),
				newAPIService("user.openshift.io", "v1"),
			},
			resultList: []string{
				"v1.authorization.openshift.io",
				"v1.build.openshift.io",
				"v1.image.openshift.io",
				"v1.oauth.openshift.io",
				"v1.route.openshift.io",
				"v1.template.openshift.io",
				"v1.user.openshift.io",
			},
			resultListChanged: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			changed, newAPIServicesSet := apiServicesChanged(tc.oldAPIServices, tc.newAPIServices)

			if changed != tc.resultListChanged {
				t.Errorf("result list chaned = %v, expected it to change = %v", changed, tc.resultListChanged)
			}

			if !equality.Semantic.DeepEqual(tc.resultList, newAPIServicesSet.List()) {
				t.Errorf("incorect api services list returned: %s", diff.ObjectDiff(tc.resultList, newAPIServicesSet.List()))
			}
		})
	}
}

func TestHandlingControlOverTheAPI(t *testing.T) {
	const externalServerAnnotation = "authentication.operator.openshift.io/managed"

	testCases := []struct {
		name                    string
		existingAPIServices     []runtime.Object
		expectedActions         []string
		expectedEventMsg        string
		expectsEvent            bool
		authOperatorUnavailable bool
		authOperatorManages     bool
	}{
		// scenario 1
		{
			name:                "checks if user.openshift.io and oauth.openshift.io groups are managed by an external server if all preconditions (authentication-operator status field set, annotations added) are valid",
			authOperatorManages: true,
			existingAPIServices: []runtime.Object{
				runtime.Object(newAPIService("build.openshift.io", "v1")),
				runtime.Object(newAPIService("apps.openshift.io", "v1")),
				runtime.Object(func() *apiregistrationv1.APIService {
					apiService := newAPIService("user.openshift.io", "v1")
					apiService.Annotations = map[string]string{}
					apiService.Annotations[externalServerAnnotation] = "true"
					return apiService
				}()),
				runtime.Object(func() *apiregistrationv1.APIService {
					apiService := newAPIService("oauth.openshift.io", "v1")
					apiService.Annotations = map[string]string{}
					apiService.Annotations[externalServerAnnotation] = "true"
					return apiService
				}()),
			},
			expectedActions:  []string{"get:apiservices:v1.user.openshift.io", "get:apiservices:v1.oauth.openshift.io", "get:apiservices:v1.build.openshift.io", "update:apiservices:v1.build.openshift.io", "get:apiservices:v1.apps.openshift.io", "update:apiservices:v1.apps.openshift.io"},
			expectedEventMsg: "The new API Services list this operator will manage is [v1.apps.openshift.io v1.build.openshift.io]",
		},

		// scenario 2
		{
			name:                "checks that oauth.openshift.io group is not managed by an external server if it's missing the annotation",
			authOperatorManages: true,
			existingAPIServices: []runtime.Object{
				runtime.Object(newAPIService("build.openshift.io", "v1")),
				runtime.Object(newAPIService("apps.openshift.io", "v1")),
				runtime.Object(func() *apiregistrationv1.APIService {
					apiService := newAPIService("user.openshift.io", "v1")
					apiService.Annotations = map[string]string{}
					apiService.Annotations[externalServerAnnotation] = "true"
					return apiService
				}()),
				runtime.Object(newAPIService("oauth.openshift.io", "v1")),
			},
			expectedActions:  []string{"get:apiservices:v1.user.openshift.io", "get:apiservices:v1.oauth.openshift.io", "get:apiservices:v1.build.openshift.io", "update:apiservices:v1.build.openshift.io", "get:apiservices:v1.apps.openshift.io", "update:apiservices:v1.apps.openshift.io", "get:apiservices:v1.oauth.openshift.io", "update:apiservices:v1.oauth.openshift.io"},
			expectedEventMsg: "The new API Services list this operator will manage is [v1.apps.openshift.io v1.build.openshift.io v1.oauth.openshift.io]",
		},

		// scenario 3
		{
			name:                "authoritative/initial list is taken if authentication-operator wasn't found BUT the annotations were added",
			authOperatorManages: true,
			existingAPIServices: []runtime.Object{
				runtime.Object(newAPIService("build.openshift.io", "v1")),
				runtime.Object(newAPIService("apps.openshift.io", "v1")),
				runtime.Object(func() *apiregistrationv1.APIService {
					apiService := newAPIService("user.openshift.io", "v1")
					apiService.Annotations = map[string]string{}
					apiService.Annotations[externalServerAnnotation] = "true"
					return apiService
				}()),
				runtime.Object(func() *apiregistrationv1.APIService {
					apiService := newAPIService("oauth.openshift.io", "v1")
					apiService.Annotations = map[string]string{}
					apiService.Annotations[externalServerAnnotation] = "true"
					return apiService
				}()),
			},
			expectedActions:         []string{"get:apiservices:v1.build.openshift.io", "update:apiservices:v1.build.openshift.io", "get:apiservices:v1.apps.openshift.io", "update:apiservices:v1.apps.openshift.io", "get:apiservices:v1.user.openshift.io", "update:apiservices:v1.user.openshift.io", "get:apiservices:v1.oauth.openshift.io", "update:apiservices:v1.oauth.openshift.io"},
			authOperatorUnavailable: true,
		},

		// scenario 4
		{
			name:                "authoritative/initial list is taken when ManagingOAuthAPIServer field set to false",
			authOperatorManages: false,
			existingAPIServices: []runtime.Object{
				runtime.Object(newAPIService("build.openshift.io", "v1")),
				runtime.Object(newAPIService("apps.openshift.io", "v1")),
				runtime.Object(func() *apiregistrationv1.APIService {
					apiService := newAPIService("user.openshift.io", "v1")
					apiService.Annotations = map[string]string{}
					apiService.Annotations[externalServerAnnotation] = "true"
					return apiService
				}()),
				runtime.Object(func() *apiregistrationv1.APIService {
					apiService := newAPIService("oauth.openshift.io", "v1")
					apiService.Annotations = map[string]string{}
					apiService.Annotations[externalServerAnnotation] = "true"
					return apiService
				}()),
			},
			expectedActions: []string{"get:apiservices:v1.build.openshift.io", "update:apiservices:v1.build.openshift.io", "get:apiservices:v1.apps.openshift.io", "update:apiservices:v1.apps.openshift.io", "get:apiservices:v1.user.openshift.io", "update:apiservices:v1.user.openshift.io", "get:apiservices:v1.oauth.openshift.io", "update:apiservices:v1.oauth.openshift.io"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			eventRecorder := events.NewInMemoryRecorder("")
			kubeClient := fake.NewSimpleClientset()
			kubeAggregatorClient := kubeaggregatorfake.NewSimpleClientset(tc.existingAPIServices...)

			fakeOperatorClient := operatorv1helpers.NewFakeOperatorClient(&operatorv1.OperatorSpec{ManagementState: operatorv1.Managed}, &operatorv1.OperatorStatus{}, nil)

			fakeAuthOperatorIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			{
				authOperator := &operatorv1.Authentication{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
					Spec:       operatorv1.AuthenticationSpec{OperatorSpec: operatorv1.OperatorSpec{ManagementState: operatorv1.Managed}},
					Status:     operatorv1.AuthenticationStatus{ManagingOAuthAPIServer: tc.authOperatorManages, OperatorStatus: operatorv1.OperatorStatus{}},
				}

				if !tc.authOperatorUnavailable {
					err := fakeAuthOperatorIndexer.Add(authOperator)
					if err != nil {
						t.Fatal(err)
					}
				}
			}

			apiServices := []*apiregistrationv1.APIService{}
			for _, rawService := range tc.existingAPIServices {
				service, ok := rawService.(*apiregistrationv1.APIService)
				if !ok {
					t.Fatal("unable to convert an api service to *apiregistrationv1.APIService")
				}
				apiServices = append(apiServices, service)
			}

			operator := &APIServiceController{
				precondition:            func([]*apiregistrationv1.APIService) (bool, error) { return true, nil },
				kubeClient:              kubeClient,
				eventRecorder:           eventRecorder,
				operatorClient:          fakeOperatorClient,
				apiregistrationv1Client: kubeAggregatorClient.ApiregistrationV1(),
				getAPIServicesToManageFn: NewAPIServicesToManage(
					kubeAggregatorClient.ApiregistrationV1(),
					operatorlistersv1.NewAuthenticationLister(fakeAuthOperatorIndexer),
					apiServices,
					eventRecorder,
					sets.NewString("v1.oauth.openshift.io", "v1.user.openshift.io"),
					externalServerAnnotation,
				).GetAPIServicesToManage,
			}

			err := operator.sync()
			if err != nil {
				t.Fatal(err)
			}

			if err := validateActionsVerbs(kubeAggregatorClient.Actions(), tc.expectedActions); err != nil {
				t.Fatal(err)
			}

			eventValidated := false
			for _, ev := range eventRecorder.Events() {
				if ev.Reason == "APIServicesToManageChanged" {
					if ev.Message != tc.expectedEventMsg {
						t.Errorf("unexpected APIServicesToManageChanged event message = %v, expected = %v", tc.expectedEventMsg, ev.Message)
					}
					eventValidated = true
				}
			}
			if !eventValidated && tc.expectsEvent {
				t.Error("APIServicesToManageChanged hasn't been found")
			}
		})
	}
}

func TestAvailableStatus(t *testing.T) {
	testCases := []struct {
		name                string
		expectedStatus      operatorv1.ConditionStatus
		expectedReasons     []string
		expectedMessages    []string
		existingAPIServices []runtime.Object
		apiServiceReactor   kubetesting.ReactionFunc
		daemonReactor       kubetesting.ReactionFunc
	}{
		{
			name:           "Default",
			expectedStatus: operatorv1.ConditionTrue,
		},
		{
			name:             "APIServiceCreateFailure",
			expectedStatus:   operatorv1.ConditionFalse,
			expectedReasons:  []string{"Error"},
			expectedMessages: []string{"TEST ERROR: fail to create apiservice"},

			apiServiceReactor: func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
				if action.GetVerb() != "create" {
					return false, nil, nil
				}
				if action.(kubetesting.CreateAction).GetObject().(*apiregistrationv1.APIService).Name == "v1.build.openshift.io" {
					return true, nil, fmt.Errorf("TEST ERROR: fail to create apiservice")
				}
				return false, nil, nil
			},
		},
		{
			name:             "APIServiceGetFailure",
			expectedStatus:   operatorv1.ConditionFalse,
			expectedReasons:  []string{"Error"},
			expectedMessages: []string{"TEST ERROR: fail to get apiservice"},

			existingAPIServices: []runtime.Object{
				runtime.Object(newAPIService("build.openshift.io", "v1")),
				runtime.Object(newAPIService("apps.openshift.io", "v1")),
			},
			apiServiceReactor: func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
				if action.GetVerb() == "get" && action.(kubetesting.GetAction).GetName() == "v1.build.openshift.io" {
					return true, nil, fmt.Errorf("TEST ERROR: fail to get apiservice")
				}
				return false, nil, nil
			},
		},
		{
			name:             "APIServiceNotAvailable",
			expectedStatus:   operatorv1.ConditionFalse,
			expectedReasons:  []string{"Error"},
			expectedMessages: []string{"apiservices.apiregistration.k8s.io/v1.build.openshift.io: not available: TEST MESSAGE"},

			existingAPIServices: []runtime.Object{
				runtime.Object(newAPIService("build.openshift.io", "v1")),
				runtime.Object(newAPIService("apps.openshift.io", "v1")),
			},
			apiServiceReactor: func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
				if action.GetVerb() == "get" && action.(kubetesting.GetAction).GetName() == "v1.build.openshift.io" {
					return true, &apiregistrationv1.APIService{
						ObjectMeta: metav1.ObjectMeta{Name: "v1.build.openshift.io", Annotations: map[string]string{"service.alpha.openshift.io/inject-cabundle": "true"}},
						Spec: apiregistrationv1.APIServiceSpec{
							Group:                "build.openshift.io",
							Version:              "v1",
							Service:              &apiregistrationv1.ServiceReference{Namespace: "target-namespace", Name: "api"},
							GroupPriorityMinimum: 9900,
							VersionPriority:      15,
						},
						Status: apiregistrationv1.APIServiceStatus{
							Conditions: []apiregistrationv1.APIServiceCondition{
								{Type: apiregistrationv1.Available, Status: apiregistrationv1.ConditionFalse, Message: "TEST MESSAGE"},
							},
						},
					}, nil
				}
				return false, nil, nil
			},
		},
		{
			name:            "MultipleAPIServiceNotAvailable",
			expectedStatus:  operatorv1.ConditionFalse,
			expectedReasons: []string{"Error"},
			expectedMessages: []string{
				"apiservices.apiregistration.k8s.io/v1.apps.openshift.io: not available: TEST MESSAGE",
				"apiservices.apiregistration.k8s.io/v1.build.openshift.io: not available: TEST MESSAGE",
			},

			existingAPIServices: []runtime.Object{
				runtime.Object(newAPIService("build.openshift.io", "v1")),
				runtime.Object(newAPIService("apps.openshift.io", "v1")),
			},
			apiServiceReactor: func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
				if action.GetVerb() != "get" {
					return false, nil, nil
				}

				switch action.(kubetesting.GetAction).GetName() {
				case "v1.build.openshift.io":
					fallthrough
				case "v1.apps.openshift.io":
					return true, &apiregistrationv1.APIService{
						ObjectMeta: metav1.ObjectMeta{Name: action.(kubetesting.GetAction).GetName(), Annotations: map[string]string{"service.alpha.openshift.io/inject-cabundle": "true"}},
						Spec: apiregistrationv1.APIServiceSpec{
							Group:                action.GetResource().Group,
							Version:              action.GetResource().Version,
							Service:              &apiregistrationv1.ServiceReference{Namespace: "target-namespace", Name: "api"},
							GroupPriorityMinimum: 9900,
							VersionPriority:      15,
						},
						Status: apiregistrationv1.APIServiceStatus{
							Conditions: []apiregistrationv1.APIServiceCondition{
								{Type: apiregistrationv1.Available, Status: apiregistrationv1.ConditionFalse, Message: "TEST MESSAGE"},
							},
						},
					}, nil
				default:
					return false, nil, nil
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			kubeClient := fake.NewSimpleClientset()
			kubeAggregatorClient := kubeaggregatorfake.NewSimpleClientset(tc.existingAPIServices...)
			if tc.apiServiceReactor != nil {
				kubeAggregatorClient.PrependReactor("*", "apiservices", tc.apiServiceReactor)
			}

			eventRecorder := events.NewInMemoryRecorder("")
			fakeOperatorClient := operatorv1helpers.NewFakeOperatorClient(&operatorv1.OperatorSpec{ManagementState: operatorv1.Managed}, &operatorv1.OperatorStatus{}, nil)
			fakeAuthOperatorIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			{
				authOperator := &operatorv1.Authentication{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
					Spec:       operatorv1.AuthenticationSpec{OperatorSpec: operatorv1.OperatorSpec{ManagementState: operatorv1.Managed}},
					Status:     operatorv1.AuthenticationStatus{OperatorStatus: operatorv1.OperatorStatus{}},
				}

				err := fakeAuthOperatorIndexer.Add(authOperator)
				if err != nil {
					t.Fatal(err)
				}
			}
			operator := &APIServiceController{
				precondition:            func([]*apiregistrationv1.APIService) (bool, error) { return true, nil },
				kubeClient:              kubeClient,
				eventRecorder:           eventRecorder,
				operatorClient:          fakeOperatorClient,
				apiregistrationv1Client: kubeAggregatorClient.ApiregistrationV1(),
				getAPIServicesToManageFn: NewAPIServicesToManage(
					kubeAggregatorClient.ApiregistrationV1(),
					operatorlistersv1.NewAuthenticationLister(fakeAuthOperatorIndexer),
					[]*apiregistrationv1.APIService{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "v1.apps.openshift.io"},
							Spec:       apiregistrationv1.APIServiceSpec{Group: "apps.openshift.io", Version: "v1", Service: &apiregistrationv1.ServiceReference{}},
						},
						{
							ObjectMeta: metav1.ObjectMeta{Name: "v1.build.openshift.io"},
							Spec:       apiregistrationv1.APIServiceSpec{Group: "build.openshift.io", Version: "v1", Service: &apiregistrationv1.ServiceReference{}},
						},
					},
					eventRecorder,
					sets.NewString("v1.oauth.openshift.io", "v1.user.openshift.io"),
					"cluster.authentication.operator.openshift.io/managed",
				).GetAPIServicesToManage,
			}

			_ = operator.sync()

			_, resultStatus, _, err := fakeOperatorClient.GetOperatorState()
			if err != nil {
				t.Fatal(err)
			}
			condition := operatorv1helpers.FindOperatorCondition(resultStatus.Conditions, "APIServicesAvailable")
			if condition == nil {
				t.Fatal("APIServicesAvailable condition not found")
			}
			if condition.Status != tc.expectedStatus {
				t.Error(diff.ObjectGoPrintSideBySide(condition.Status, tc.expectedStatus))
			}
			expectedReasons := strings.Join(tc.expectedReasons, "\n")
			if len(expectedReasons) > 0 && condition.Reason != expectedReasons {
				t.Error(diff.ObjectGoPrintSideBySide(condition.Reason, expectedReasons))
			}
			if len(tc.expectedMessages) > 0 {
				actualMessages := strings.Split(condition.Message, "\n")
				a := make([]string, len(tc.expectedMessages))
				b := make([]string, len(actualMessages))
				copy(a, tc.expectedMessages)
				copy(b, actualMessages)
				sort.Strings(a)
				sort.Strings(b)
				if !equality.Semantic.DeepEqual(a, b) {
					t.Error("\n" + diff.ObjectDiff(a, b))
				}
			}
		})
	}

}

func newAPIService(group, version string) *apiregistrationv1.APIService {
	return &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{Name: version + "." + group, Annotations: map[string]string{"service.alpha.openshift.io/inject-cabundle": "true"}},
		Spec:       apiregistrationv1.APIServiceSpec{Group: group, Version: version, Service: &apiregistrationv1.ServiceReference{Namespace: "target-namespace", Name: "api"}, GroupPriorityMinimum: 9900, VersionPriority: 15},
		Status:     apiregistrationv1.APIServiceStatus{Conditions: []apiregistrationv1.APIServiceCondition{{Type: apiregistrationv1.Available, Status: apiregistrationv1.ConditionTrue}}},
	}
}

func validateActionsVerbs(actualActions []clientgotesting.Action, expectedActions []string) error {
	if len(actualActions) != len(expectedActions) {
		return fmt.Errorf("expected to get %d actions but got %d\nexpected=%v \n got=%v", len(expectedActions), len(actualActions), expectedActions, actionStrings(actualActions))
	}
	for i, a := range actualActions {
		if got, expected := actionString(a), expectedActions[i]; got != expected {
			return fmt.Errorf("at %d got %s, expected %s", i, got, expected)
		}
	}
	return nil
}

func actionString(a clientgotesting.Action) string {
	involvedObjectName := ""
	if updateAction, isUpdateAction := a.(clientgotesting.UpdateAction); isUpdateAction {
		rawObj := updateAction.GetObject()
		if objMeta, err := meta.Accessor(rawObj); err == nil {
			involvedObjectName = objMeta.GetName()
		}
	}
	if getAction, isGetAction := a.(clientgotesting.GetAction); isGetAction {
		involvedObjectName = getAction.GetName()
	}
	action := a.GetVerb() + ":" + a.GetResource().Resource
	if len(a.GetNamespace()) > 0 {
		action = action + ":" + a.GetNamespace()
	}
	if len(involvedObjectName) > 0 {
		action = action + ":" + involvedObjectName
	}
	return action
}

func actionStrings(actions []clientgotesting.Action) []string {
	res := make([]string, 0, len(actions))
	for _, a := range actions {
		res = append(res, actionString(a))
	}
	return res
}
