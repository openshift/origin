package templates

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kappsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	deploymentutil "k8s.io/kubernetes/pkg/controller/deployment/util"

	appsv1 "github.com/openshift/api/apps/v1"
	authorizationv1 "github.com/openshift/api/authorization/v1"
	buildv1 "github.com/openshift/api/build/v1"
	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	userv1 "github.com/openshift/api/user/v1"

	"github.com/openshift/library-go/pkg/apps/appsutil"

	buildv1client "github.com/openshift/client-go/build/clientset/versioned"

	osbclient "github.com/openshift/origin/test/extended/templates/openservicebroker/client"

	exutil "github.com/openshift/origin/test/extended/util"
)

var readinessScheme = runtime.NewScheme()

func init() {
	kappsv1.AddToScheme(readinessScheme)
	batchv1.AddToScheme(readinessScheme)
	corev1.AddToScheme(readinessScheme)
	appsv1.Install(readinessScheme)
	buildv1.Install(readinessScheme)
	routev1.Install(readinessScheme)
}

func createUser(cli *exutil.CLI, name, role string) *userv1.User {
	name = cli.Namespace() + "-" + name

	user, err := cli.AdminUserClient().UserV1().Users().Create(context.Background(), &userv1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if role != "" {
		_, err = cli.AdminAuthorizationClient().AuthorizationV1().RoleBindings(cli.Namespace()).Create(context.Background(), &authorizationv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%s-binding", name, role),
			},
			RoleRef: corev1.ObjectReference{
				Name: role,
			},
			Subjects: []corev1.ObjectReference{
				{
					Kind: authorizationv1.UserKind,
					Name: name,
				},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	return user
}

func createGroup(cli *exutil.CLI, name, role string) *userv1.Group {
	name = cli.Namespace() + "-" + name

	group, err := cli.AdminUserClient().UserV1().Groups().Create(context.Background(), &userv1.Group{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if role != "" {
		_, err = cli.AdminAuthorizationClient().AuthorizationV1().RoleBindings(cli.Namespace()).Create(context.Background(), &authorizationv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%s-binding", name, role),
			},
			RoleRef: corev1.ObjectReference{
				Name: role,
			},
			Subjects: []corev1.ObjectReference{
				{
					Kind: authorizationv1.GroupKind,
					Name: name,
				},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	return group
}

func addUserToGroup(cli *exutil.CLI, username, groupname string) {
	group, err := cli.AdminUserClient().UserV1().Groups().Get(context.Background(), groupname, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if group != nil {
		group.Users = append(group.Users, username)
		_, err = cli.AdminUserClient().UserV1().Groups().Update(context.Background(), group, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func deleteGroup(cli *exutil.CLI, group *userv1.Group) {
	err := cli.AdminUserClient().UserV1().Groups().Delete(context.Background(), group.Name, metav1.DeleteOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func deleteUser(cli *exutil.CLI, user *userv1.User) {
	err := cli.AdminUserClient().UserV1().Users().Delete(context.Background(), user.Name, metav1.DeleteOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func setUser(cli *exutil.CLI, user *userv1.User) {
	if user == nil {
		g.By("testing as system:admin user")
		*cli = *cli.AsAdmin()
	} else {
		g.By(fmt.Sprintf("testing as %s user", user.Name))
		cli.ChangeUser(user.Name)
	}
}

// TSBClient returns a client to the running template service broker
func TSBClient(oc *exutil.CLI) (osbclient.Client, error) {
	svc, err := oc.AdminKubeClient().CoreV1().Services("openshift-template-service-broker").Get(context.Background(), "apiserver", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return osbclient.NewClient(&http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}, "https://"+svc.Spec.ClusterIP+"/brokers/template.openshift.io"), nil
}

// readinessCheckers maps GroupKinds to the appropriate function.  Note that in
// some cases more than one GK maps to the same function.
var readinessCheckers = map[schema.GroupVersionKind]func(runtime.Object) (bool, bool, error){
	// OpenShift kinds:
	groupVersionKind(buildv1.GroupVersion, "Build"):           checkBuildReadiness,
	groupVersionKind(appsv1.GroupVersion, "DeploymentConfig"): checkDeploymentConfigReadiness,
	groupVersionKind(routev1.GroupVersion, "Route"):           checkRouteReadiness,

	// Legacy (/oapi) kinds:
	{Group: "", Version: "v1", Kind: "Build"}:            checkBuildReadiness,
	{Group: "", Version: "v1", Kind: "DeploymentConfig"}: checkDeploymentConfigReadiness,
	{Group: "", Version: "v1", Kind: "Route"}:            checkRouteReadiness,

	// Kubernetes kinds:
	groupVersionKind(kappsv1.SchemeGroupVersion, "Deployment"):  checkDeploymentReadiness,
	groupVersionKind(kappsv1.SchemeGroupVersion, "StatefulSet"): checkStatefulSetReadiness,
	groupVersionKind(batchv1.SchemeGroupVersion, "Job"):         checkJobReadiness,
}

//TODO candidate for openshift/library-go
func isTerminalPhase(phase buildv1.BuildPhase) bool {
	switch phase {
	case buildv1.BuildPhaseNew,
		buildv1.BuildPhasePending,
		buildv1.BuildPhaseRunning:
		return false
	}
	return true
}

//TODO candidate for openshift/library-go
func checkBuildReadiness(obj runtime.Object) (bool, bool, error) {
	b, ok := obj.(*buildv1.Build)
	if !ok {
		return false, false, fmt.Errorf("object %T is not v1.Build", obj)
	}

	ready := isTerminalPhase(b.Status.Phase) &&
		b.Status.Phase == buildv1.BuildPhaseComplete

	failed := isTerminalPhase(b.Status.Phase) &&
		b.Status.Phase != buildv1.BuildPhaseComplete

	return ready, failed, nil
}

//TODO candidate for openshift/library-go
func labelValue(name string) string {
	if len(name) <= validation.DNS1123LabelMaxLength {
		return name
	}
	return name[:validation.DNS1123LabelMaxLength]
}

//TODO candidate for openshift/library-go
func buildConfigSelector(name string) labels.Selector {
	return labels.Set{buildv1.BuildConfigLabel: labelValue(name)}.AsSelector()
}

func checkBuildConfigReadiness(oc buildv1client.Interface, obj runtime.Object) (bool, bool, error) {
	bc, ok := obj.(*buildv1.BuildConfig)
	if !ok {
		return false, false, fmt.Errorf("object %T is not v1.BuildConfig", obj)
	}

	builds, err := oc.BuildV1().Builds(bc.Namespace).List(context.Background(), metav1.ListOptions{LabelSelector: buildConfigSelector(bc.Name).String()})
	if err != nil {
		return false, false, err
	}

	for _, item := range builds.Items {
		if item.Annotations[buildv1.BuildNumberAnnotation] == strconv.FormatInt(bc.Status.LastVersion, 10) {
			return checkBuildReadiness(&item)
		}
	}

	return false, false, nil
}

type deploymentCondition struct {
	status corev1.ConditionStatus
	reason string
}

func newDeploymentCondition(status corev1.ConditionStatus, reason string) *deploymentCondition {
	return &deploymentCondition{
		status: status,
		reason: reason,
	}
}

// checkDeploymentReadiness determins if a Deployment is ready, failed or
// neither.
func checkDeploymentReadiness(obj runtime.Object) (bool, bool, error) {
	var (
		isSynced               bool
		progressing, available *deploymentCondition
	)
	switch d := obj.(type) {
	case *kappsv1.Deployment:
		isSynced = d.Status.ObservedGeneration == d.Generation
		for _, condition := range d.Status.Conditions {
			switch condition.Type {
			case kappsv1.DeploymentProgressing:
				progressing = newDeploymentCondition(condition.Status, condition.Reason)
			case kappsv1.DeploymentAvailable:
				available = newDeploymentCondition(condition.Status, condition.Reason)
			}
		}
	default:
		return false, false, fmt.Errorf("unsupported deployment version: %T", d)
	}

	if !isSynced || progressing == nil {
		return false, false, nil
	}

	ready := progressing.status == corev1.ConditionTrue &&
		progressing.reason == deploymentutil.NewRSAvailableReason &&
		available != nil &&
		available.status == corev1.ConditionTrue

	failed := progressing.status == corev1.ConditionFalse

	return ready, failed, nil
}

//TODO candidate for openshift/library-go
func checkDeploymentConfigReadiness(obj runtime.Object) (bool, bool, error) {
	dc, ok := obj.(*appsv1.DeploymentConfig)
	if !ok {
		return false, false, fmt.Errorf("object %T is not v1.DeploymentConfig", obj)
	}

	var progressing, available *appsv1.DeploymentCondition
	for i, condition := range dc.Status.Conditions {
		switch condition.Type {
		case appsv1.DeploymentProgressing:
			progressing = &dc.Status.Conditions[i]

		case appsv1.DeploymentAvailable:
			available = &dc.Status.Conditions[i]
		}
	}

	ready := dc.Status.ObservedGeneration == dc.Generation &&
		progressing != nil &&
		progressing.Status == corev1.ConditionTrue &&
		progressing.Reason == appsutil.NewRcAvailableReason &&
		available != nil &&
		available.Status == corev1.ConditionTrue

	failed := dc.Status.ObservedGeneration == dc.Generation &&
		progressing != nil &&
		progressing.Status == corev1.ConditionFalse

	return ready, failed, nil
}

func checkJobReadiness(obj runtime.Object) (bool, bool, error) {
	var (
		hasCompletionTime bool
		isJobFailed       bool
	)
	switch j := obj.(type) {
	case *batchv1.Job:
		hasCompletionTime = j.Status.CompletionTime != nil
		isJobFailed = j.Status.Failed > 0
	default:
		return false, false, fmt.Errorf("unsupported job version: %T", j)
	}
	return hasCompletionTime, isJobFailed, nil
}

func checkRouteReadiness(obj runtime.Object) (bool, bool, error) {
	route, ok := obj.(*routev1.Route)
	if !ok {
		return false, false, fmt.Errorf("object %T is not v1.Route", obj)
	}
	return len(route.Spec.Host) > 0, false, nil
}

func checkStatefulSetReadiness(obj runtime.Object) (bool, bool, error) {
	var (
		isSynced         bool
		hasReplicasReady bool
	)

	switch s := obj.(type) {
	case *kappsv1.StatefulSet:
		isSynced = s.Status.ObservedGeneration == s.Generation
		hasReplicasReady = s.Spec.Replicas != nil && s.Status.ReadyReplicas == *s.Spec.Replicas
	default:
		return false, false, fmt.Errorf("unsupported statefulset version: %T", s)
	}

	return isSynced && hasReplicasReady, false, nil
}

func groupVersionKind(gv schema.GroupVersion, kind string) schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    kind,
	}
}

func canCheckReadiness(ref corev1.ObjectReference) bool {
	switch ref.GroupVersionKind() {
	case groupVersionKind(buildv1.GroupVersion, "BuildConfig"), schema.GroupVersionKind{Group: "", Version: "v1", Kind: "BuildConfig"}:
		return true
	}
	_, found := readinessCheckers[ref.GroupVersionKind()]
	return found
}

func checkReadiness(oc buildv1client.Interface, ref corev1.ObjectReference, obj *unstructured.Unstructured) (bool, bool, error) {
	castObj, err := readinessScheme.New(ref.GroupVersionKind())
	if err != nil {
		return false, false, err
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, castObj); err != nil {
		return false, false, err
	}

	switch ref.GroupVersionKind() {
	case groupVersionKind(buildv1.GroupVersion, "BuildConfig"), schema.GroupVersionKind{Group: "", Version: "v1", Kind: "BuildConfig"}:
		return checkBuildConfigReadiness(oc, castObj)
	}

	readinessCheckFunc, ok := readinessCheckers[ref.GroupVersionKind()]
	if !ok {
		return false, false, fmt.Errorf("readiness check for %+v is not defined", ref.GroupVersionKind())
	}
	return readinessCheckFunc(castObj)
}

func dumpObjectReadiness(oc *exutil.CLI, templateInstance *templatev1.TemplateInstance) error {
	restmapper := oc.RESTMapper()

	fmt.Fprintf(g.GinkgoWriter, "dumping object readiness for %s/%s\n", templateInstance.Namespace, templateInstance.Name)

	for _, object := range templateInstance.Status.Objects {
		if !canCheckReadiness(object.Ref) {
			continue
		}

		mapping, err := restmapper.RESTMapping(object.Ref.GroupVersionKind().GroupKind())
		if err != nil {
			return err
		}

		obj, err := oc.KubeFramework().DynamicClient.Resource(mapping.Resource).Namespace(object.Ref.Namespace).Get(context.Background(), object.Ref.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if obj.GetUID() != object.Ref.UID {
			return kerrors.NewNotFound(mapping.Resource.GroupResource(), object.Ref.Name)
		}

		if strings.ToLower(obj.GetAnnotations()[templatev1.WaitForReadyAnnotation]) != "true" {
			continue
		}

		ready, failed, err := checkReadiness(oc.BuildClient(), object.Ref, obj)
		if err != nil {
			return err
		}

		fmt.Fprintf(g.GinkgoWriter, "%s %s/%s: ready %v, failed %v\n", object.Ref.Kind, object.Ref.Namespace, object.Ref.Name, ready, failed)
		if !ready || failed {
			fmt.Fprintf(g.GinkgoWriter, "object: %#v\n", obj)
		}
	}

	return nil
}
