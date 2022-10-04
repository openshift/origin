package ingress

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	iov1 "github.com/openshift/api/operatoringress/v1"
	"github.com/openshift/cluster-ingress-operator/pkg/manifests"
	"github.com/openshift/cluster-ingress-operator/pkg/operator/controller"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// defaultRecordTTL is the TTL (in seconds) assigned to all new DNS records.
//
// Note that TTL isn't necessarily honored by clouds providers (for example,
// on AWS TTL is not configurable for alias records[1]).
//
// [1] https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/resource-record-sets-choosing-alias-non-alias.html
const defaultRecordTTL int64 = 30

// ensureWildcardDNSRecord will create DNS records for the given LB service.
// If service is nil (haveLBS is false), nothing is done.
func (r *reconciler) ensureWildcardDNSRecord(ic *operatorv1.IngressController, service *corev1.Service, haveLBS bool) (bool, *iov1.DNSRecord, error) {
	if !haveLBS {
		return false, nil, nil
	}

	wantWC, desired := desiredWildcardDNSRecord(ic, service)
	haveWC, current, err := r.currentWildcardDNSRecord(ic)
	if err != nil {
		return false, nil, err
	}

	switch {
	case wantWC && !haveWC:
		if err := r.client.Create(context.TODO(), desired); err != nil {
			return false, nil, fmt.Errorf("failed to create dnsrecord %s/%s: %v", desired.Namespace, desired.Name, err)
		}
		log.Info("created dnsrecord", "dnsrecord", desired)
		return r.currentWildcardDNSRecord(ic)
	case wantWC && haveWC:
		if updated, err := r.updateDNSRecord(current, desired); err != nil {
			return true, current, fmt.Errorf("failed to update dnsrecord %s/%s: %v", desired.Namespace, desired.Name, err)
		} else if updated {
			return r.currentWildcardDNSRecord(ic)
		}
	}

	return haveWC, current, nil
}

// desiredWildcardDNSRecord will return any necessary wildcard DNS records for the
// ingresscontroller.
//
// For now, if the service has more than one .status.loadbalancer.ingress, only
// the first will be used.
//
// TODO: If .status.loadbalancer.ingress is processed once as non-empty and then
// later becomes empty, what should we do? Currently we'll treat it as an intent
// to not have a desired record.
func desiredWildcardDNSRecord(ic *operatorv1.IngressController, service *corev1.Service) (bool, *iov1.DNSRecord) {
	// If the ingresscontroller has no ingress domain, we cannot configure any
	// DNS records.
	if len(ic.Status.Domain) == 0 {
		return false, nil
	}

	// DNS is only managed for LB publishing.
	if ic.Status.EndpointPublishingStrategy.Type != operatorv1.LoadBalancerServiceStrategyType {
		return false, nil
	}

	// No LB target exists for the domain record to point at.
	if len(service.Status.LoadBalancer.Ingress) == 0 {
		return false, nil
	}

	ingress := service.Status.LoadBalancer.Ingress[0]

	// Quick sanity check since we don't know how to handle both being set (is
	// that even a valid state?)
	if len(ingress.Hostname) > 0 && len(ingress.IP) > 0 {
		return false, nil
	}

	name := controller.WildcardDNSRecordName(ic)
	// Use an absolute name to prevent any ambiguity.
	domain := fmt.Sprintf("*.%s.", ic.Status.Domain)
	var target string
	var recordType iov1.DNSRecordType

	if len(ingress.Hostname) > 0 {
		recordType = iov1.CNAMERecordType
		target = ingress.Hostname
	} else {
		recordType = iov1.ARecordType
		target = ingress.IP
	}

	dnsPolicy := iov1.ManagedDNS

	// Set the DNS management policy on the dnsrecord to "Unmanaged" if ingresscontroller has "Unmanaged" DNS policy or
	// if the ingresscontroller domain isn't a subdomain of the cluster's base domain.
	if ic.Status.EndpointPublishingStrategy.LoadBalancer.DNSManagementPolicy == operatorv1.UnmanagedLoadBalancerDNS {
		dnsPolicy = iov1.UnmanagedDNS
	}

	trueVar := true
	return true, &iov1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: name.Namespace,
			Name:      name.Name,
			Labels: map[string]string{
				manifests.OwningIngressControllerLabel: ic.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         operatorv1.GroupVersion.String(),
					Kind:               "IngressController",
					Name:               ic.Name,
					UID:                ic.UID,
					Controller:         &trueVar,
					BlockOwnerDeletion: &trueVar,
				},
			},
			Finalizers: []string{manifests.DNSRecordFinalizer},
		},
		Spec: iov1.DNSRecordSpec{
			DNSName:             domain,
			DNSManagementPolicy: dnsPolicy,
			Targets:             []string{target},
			RecordType:          recordType,
			RecordTTL:           defaultRecordTTL,
		},
	}
}

func (r *reconciler) currentWildcardDNSRecord(ic *operatorv1.IngressController) (bool, *iov1.DNSRecord, error) {
	current := &iov1.DNSRecord{}
	err := r.client.Get(context.TODO(), controller.WildcardDNSRecordName(ic), current)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, current, nil
}

func (r *reconciler) deleteWildcardDNSRecord(ic *operatorv1.IngressController) error {
	name := controller.WildcardDNSRecordName(ic)
	record := &iov1.DNSRecord{}
	record.Namespace = name.Namespace
	record.Name = name.Name
	if err := r.client.Delete(context.TODO(), record); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

// updateDNSRecord updates a DNSRecord. Returns a boolean indicating whether
// the record was updated, and an error value.
func (r *reconciler) updateDNSRecord(current, desired *iov1.DNSRecord) (bool, error) {
	changed, updated := dnsRecordChanged(current, desired)
	if !changed {
		return false, nil
	}

	// Diff before updating because the client may mutate the object.
	diff := cmp.Diff(current, updated, cmpopts.EquateEmpty())
	if err := r.client.Update(context.TODO(), updated); err != nil {
		return false, err
	}
	log.Info("updated dnsrecord", "namespace", updated.Namespace, "name", updated.Name, "diff", diff)
	return true, nil
}

// dnsRecordChanged checks if the current DNSRecord spec matches the expected spec and
// if not returns an updated one.
func dnsRecordChanged(current, expected *iov1.DNSRecord) (bool, *iov1.DNSRecord) {
	if cmp.Equal(current.Spec, expected.Spec, cmpopts.EquateEmpty()) {
		return false, nil
	}

	updated := current.DeepCopy()
	updated.Spec = expected.Spec
	return true, updated
}

// manageDNSForDomain returns true if the given domain contains the baseDomain
// of the cluster DNS config. It is only used for AWS in the beginning, and will be expanded to other clouds
// once we know there are no users depending on this.
// See https://bugzilla.redhat.com/show_bug.cgi?id=2041616
func manageDNSForDomain(domain string, status *configv1.PlatformStatus, dnsConfig *configv1.DNS) bool {
	if len(domain) == 0 {
		return false
	}

	mustContain := "." + dnsConfig.Spec.BaseDomain
	switch status.Type {
	case configv1.AWSPlatformType:
		return strings.HasSuffix(domain, mustContain)
	default:
		return true
	}
}
