/*
Copyright 2018 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package idler_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/kubernetes-sigs/kubebuilder/pkg/controller/types"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	fakescale "k8s.io/client-go/scale/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"

	idling "github.com/openshift/service-idler/pkg/apis/idling/v1alpha2"
	. "github.com/openshift/service-idler/pkg/controller/idler"
)

// okErr represents return (bool, error) in struct form
type okErr struct {
	ok  bool
	err error
}

// replicasOrErr represents responses to scale queries
type replicasOrErr struct {
	replicas int32
	err      error
}

// scalesMap saves us time on typing CrossGroupObjectReference a bajillion times.
// TODO: namespace this
type scalesMap map[idling.CrossGroupObjectReference]replicasOrErr

var (
	mildCheddar  = idling.CrossGroupObjectReference{Group: "cheese", Resource: "cheddar", Name: "mild"}
	sharpCheddar = idling.CrossGroupObjectReference{Group: "cheese", Resource: "cheddar", Name: "sharp"}
	american     = idling.CrossGroupObjectReference{Group: "cheese", Resource: "american", Name: "white"}
	ritz         = types.ReconcileKey{Name: "ritz"}
	triscuit     = types.ReconcileKey{Name: "triscuit"}
)

var _ = Describe("Idling Executor", func() {
	var exec *IdlerExecutor
	var endpointsInfo map[types.ReconcileKey]okErr
	var updatedIdler *idling.Idler
	var updateError error
	var scales scalesMap

	var idler *idling.Idler

	JustBeforeEach(func() {
		fakeClient := &fakescale.FakeScaleClient{}
		fakeClient.AddReactor("get", "*", func(action core.Action) (bool, runtime.Object, error) {
			act := action.(core.GetAction)
			gvr := act.GetResource()
			objRef := idling.CrossGroupObjectReference{
				Resource: gvr.Resource,
				Group:    gvr.Group,
				Name:     act.GetName(),
			}
			replicas, ok := scales[objRef]
			if !ok {
				return true, nil, fmt.Errorf("no such object (from scale) %s/%s", act.GetNamespace(), act.GetName())
			}

			return true, &autoscalingv1.Scale{
				Spec: autoscalingv1.ScaleSpec{
					Replicas: replicas.replicas,
				},
			}, nil
		})
		fakeClient.AddReactor("update", "*", func(action core.Action) (bool, runtime.Object, error) {
			act := action.(core.UpdateAction)
			gvr := act.GetResource()
			newScale := act.GetObject().(*autoscalingv1.Scale)
			objRef := idling.CrossGroupObjectReference{
				Resource: gvr.Resource,
				Group:    gvr.Group,
				Name:     newScale.Name,
			}
			res, ok := scales[objRef]
			if !ok {
				return true, nil, fmt.Errorf("no such object (from scale) %s/%s", act.GetNamespace(), newScale.Name)
			}
			if res.err != nil {
				return true, nil, res.err
			}
			scales[objRef] = replicasOrErr{replicas: newScale.Spec.Replicas}

			return true, &autoscalingv1.Scale{
				Spec: autoscalingv1.ScaleSpec{
					Replicas: newScale.Spec.Replicas,
				},
			}, nil
		})
		exec = &IdlerExecutor{
			ScaleClient: fakeClient,
			EndpointsActive: func(ep types.ReconcileKey) (bool, error) {
				val := endpointsInfo[ep]
				return val.ok, val.err
			},
			UpdateIdler: func(idler *idling.Idler) error {
				updatedIdler = idler
				return updateError
			},
			EventRecorder: &record.FakeRecorder{},
		}
	})

	Describe("Idling", func() {
		BeforeEach(func() {
			idler = &idling.Idler{
				Spec: idling.IdlerSpec{
					TargetScalables: []idling.CrossGroupObjectReference{
						sharpCheddar, mildCheddar, american,
					},
				},
			}
			scales = scalesMap{
				sharpCheddar: replicasOrErr{replicas: 3},
				mildCheddar:  replicasOrErr{replicas: 4},
				american:     replicasOrErr{replicas: 5},
			}
			updateError = nil
			endpointsInfo = map[types.ReconcileKey]okErr{}
			updatedIdler = nil
		})

		Context("in case of errors", func() {
			It("should continue fetching scales", func() {
				delete(scales, idling.CrossGroupObjectReference{Group: "cheese", Resource: "cheddar", Name: "mild"})
				By("executing the idling")
				errs := exec.EnsureIdle(NewCoW(idler))
				err := utilerrors.NewAggregate(errs)
				Expect(err).To(HaveOccurred())

				Expect(updatedIdler).NotTo(BeNil())
				Expect(updatedIdler.Status.Idled).To(BeTrue())
				Expect(updatedIdler.Status.UnidledScales).To(ConsistOf(
					idling.UnidleInfo{sharpCheddar, 3},
					idling.UnidleInfo{american, 5},
				))

				Expect(scales).To(Equal(scalesMap{
					sharpCheddar: replicasOrErr{},
					american:     replicasOrErr{},
				}))
			})

			It("should continue updating scales", func() {
				scales[mildCheddar] = replicasOrErr{
					replicas: 4,
					err:      fmt.Errorf("who likes mild cheddar?"),
				}

				By("executing the idling")
				errs := exec.EnsureIdle(NewCoW(idler))
				err := utilerrors.NewAggregate(errs)
				Expect(err).To(HaveOccurred())

				By("verifying the recorded status")
				Expect(updatedIdler).NotTo(BeNil())
				Expect(updatedIdler.Status.Idled).To(BeTrue())
				Expect(updatedIdler.Status.UnidledScales).To(ConsistOf(
					idling.UnidleInfo{sharpCheddar, 3},
					idling.UnidleInfo{mildCheddar, 4},
					idling.UnidleInfo{american, 5},
				))

				By("verifying the final scales")
				Expect(scales).To(HaveKeyWithValue(sharpCheddar, replicasOrErr{}))
				Expect(scales).To(HaveKeyWithValue(american, replicasOrErr{}))
			})

			It("should not update scales after failing to set idled state", func() {
				updateError = fmt.Errorf("oh no, couldn't update")

				By("executing the idling")
				errs := exec.EnsureIdle(NewCoW(idler))
				err := utilerrors.NewAggregate(errs)
				Expect(err).To(HaveOccurred())

				By("verifying the final scales")
				Expect(scales).To(Equal(scalesMap{
					sharpCheddar: replicasOrErr{replicas: 3},
					mildCheddar:  replicasOrErr{replicas: 4},
					american:     replicasOrErr{replicas: 5},
				}))
			})
		})

		Context("in case of updates to the list of target scalables", func() {
			It("should enforce the idling of new additions", func() {
				scales[sharpCheddar] = replicasOrErr{}
				scales[american] = replicasOrErr{}
				idler.Status = idling.IdlerStatus{
					Idled: true,
					UnidledScales: []idling.UnidleInfo{
						{sharpCheddar, 3},
						{american, 5},
					},
				}
				idler.Spec.TargetScalables = append(idler.Spec.TargetScalables, mildCheddar)

				By("executing the idling")
				errs := exec.EnsureIdle(NewCoW(idler))
				err := utilerrors.NewAggregate(errs)
				Expect(err).NotTo(HaveOccurred())

				By("verifying the final scales")
				Expect(scales).To(Equal(scalesMap{
					sharpCheddar: replicasOrErr{},
					mildCheddar:  replicasOrErr{},
					american:     replicasOrErr{},
				}))

				By("verifying the recorded status")
				Expect(updatedIdler.Status.Idled).To(BeTrue())
				Expect(updatedIdler.Status.UnidledScales).To(ConsistOf(
					idling.UnidleInfo{sharpCheddar, 3},
					idling.UnidleInfo{american, 5},
					idling.UnidleInfo{mildCheddar, 4},
				))
			})

			It("should preserve the newer scale value in case of conflicts", func() {
				scales[sharpCheddar] = replicasOrErr{}
				scales[american] = replicasOrErr{replicas: 6}
				idler.Status = idling.IdlerStatus{
					Idled: true,
					UnidledScales: []idling.UnidleInfo{
						{sharpCheddar, 3},
						{american, 5},
					},
				}
				idler.Spec.TargetScalables = append(idler.Spec.TargetScalables, mildCheddar)

				By("executing the idling")
				errs := exec.EnsureIdle(NewCoW(idler))
				err := utilerrors.NewAggregate(errs)
				Expect(err).NotTo(HaveOccurred())

				By("verifying the final scales")
				Expect(scales).To(Equal(scalesMap{
					sharpCheddar: replicasOrErr{},
					mildCheddar:  replicasOrErr{},
					american:     replicasOrErr{},
				}))

				By("verifying the recorded status")
				Expect(updatedIdler.Status.Idled).To(BeTrue())
				Expect(updatedIdler.Status.UnidledScales).To(ConsistOf(
					idling.UnidleInfo{sharpCheddar, 3},
					idling.UnidleInfo{american, 6},
					idling.UnidleInfo{mildCheddar, 4},
				))
			})
		})

		It("should ensure all target scalables are scaled to zero, with recorded scales", func() {
			By("executing the idling")
			errs := exec.EnsureIdle(NewCoW(idler))
			err := utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())

			By("verifying the final scales")
			Expect(scales).To(Equal(scalesMap{
				sharpCheddar: replicasOrErr{},
				mildCheddar:  replicasOrErr{},
				american:     replicasOrErr{},
			}))

			By("verifying the recorded status")
			Expect(updatedIdler.Status.Idled).To(BeTrue())
			Expect(updatedIdler.Status.UnidledScales).To(ConsistOf(
				idling.UnidleInfo{sharpCheddar, 3},
				idling.UnidleInfo{american, 5},
				idling.UnidleInfo{mildCheddar, 4},
			))
		})

		It("should ignore scalables already scaled to zero", func() {
			// set an error so that if we try to scale, we'll see an error
			scales[mildCheddar] = replicasOrErr{}

			By("executing the idling")
			errs := exec.EnsureIdle(NewCoW(idler))
			err := utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())

			By("verifying the recorded status does not contain the zero-scale target")
			Expect(updatedIdler.Status.Idled).To(BeTrue())
			Expect(updatedIdler.Status.UnidledScales).To(ConsistOf(
				idling.UnidleInfo{sharpCheddar, 3},
				idling.UnidleInfo{american, 5},
			))
		})

		It("shouldn't run updates if nothing's changed", func() {
			// set an error so that if we try to update, we'll see an error
			updateError = fmt.Errorf("you shouldn't have come here... 👻")

			scales[mildCheddar] = replicasOrErr{}
			scales[american] = replicasOrErr{}
			scales[sharpCheddar] = replicasOrErr{}
			idler.Status.UnidledScales = []idling.UnidleInfo{
				{sharpCheddar, 3},
				{mildCheddar, 4},
				{american, 5},
			}
			idler.Status.Idled = true

			By("executing the idling without hitting the trap error")
			errs := exec.EnsureIdle(NewCoW(idler))
			err := utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Unidling", func() {
		BeforeEach(func() {
			idler = &idling.Idler{
				Spec: idling.IdlerSpec{
					TargetScalables: []idling.CrossGroupObjectReference{
						sharpCheddar, mildCheddar, american,
					},
					TriggerServiceNames: []string{ritz.Name, triscuit.Name},
				},
				Status: idling.IdlerStatus{
					Idled: true,
					UnidledScales: []idling.UnidleInfo{
						{sharpCheddar, 3},
						{mildCheddar, 4},
						{american, 5},
					},
				},
			}
			scales = scalesMap{
				sharpCheddar: replicasOrErr{},
				mildCheddar:  replicasOrErr{},
				american:     replicasOrErr{},
			}
			updateError = nil
			endpointsInfo = map[types.ReconcileKey]okErr{}
			updatedIdler = nil
		})
		Context("in case of errors", func() {
			It("should continue updating scales", func() {
				scales[mildCheddar] = replicasOrErr{err: fmt.Errorf("you have no chance to survive make your time")}

				cow := NewCoW(idler)

				By("populating the inactive services")
				errs := exec.PopulateInactiveServices(cow)
				err := utilerrors.NewAggregate(errs)
				Expect(err).NotTo(HaveOccurred())

				By("executing the unidling")
				errs = exec.EnsureUnidle(cow)
				err = utilerrors.NewAggregate(errs)
				Expect(err).To(HaveOccurred())

				By("verifying that the other scalables are scaled back up")
				Expect(scales).To(HaveKeyWithValue(sharpCheddar, replicasOrErr{replicas: 3}))
				Expect(scales).To(HaveKeyWithValue(american, replicasOrErr{replicas: 5}))
			})

			It("should keep around the previous scale for failed updates", func() {
				scales[mildCheddar] = replicasOrErr{err: fmt.Errorf("you have no chance to survive make your time")}
				cow := NewCoW(idler)

				By("populating the inactive services")
				errs := exec.PopulateInactiveServices(cow)
				err := utilerrors.NewAggregate(errs)
				Expect(err).NotTo(HaveOccurred())

				By("executing the unidling")
				errs = exec.EnsureUnidle(cow)
				err = utilerrors.NewAggregate(errs)
				Expect(err).To(HaveOccurred())

				By("verifying the idling status still contains the scale for the failed scalable")
				Expect(updatedIdler.Status.UnidledScales).To(ConsistOf(
					idling.UnidleInfo{mildCheddar, 4},
				))
			})
		})

		It("should not touch any target scalables without recorded scales", func() {
			scales[mildCheddar] = replicasOrErr{replicas: 27}
			idler.Status.UnidledScales = []idling.UnidleInfo{
				{sharpCheddar, 3},
				{american, 5},
			}
			cow := NewCoW(idler)

			By("populating the inactive services")
			errs := exec.PopulateInactiveServices(cow)
			err := utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())

			By("executing the unidling")
			errs = exec.EnsureUnidle(cow)
			err = utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that the scale was not changed")
			Expect(scales).To(HaveKeyWithValue(mildCheddar, replicasOrErr{replicas: 27}))
		})

		It("should clean up any recorded scales which don't have corresponding target scalables", func() {
			idler.Status.UnidledScales = append(idler.Status.UnidledScales,
				idling.UnidleInfo{idling.CrossGroupObjectReference{Name: "foobar"}, 27})

			cow := NewCoW(idler)

			By("populating the inactive services")
			errs := exec.PopulateInactiveServices(cow)
			err := utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())

			By("executing the unidling")
			errs = exec.EnsureUnidle(cow)
			err = utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that the previous scale is gone")
			Expect(updatedIdler.Status.UnidledScales).To(BeEmpty())
		})

		It("should set idled status to false if all trigger services have endpoints", func() {
			endpointsInfo = map[types.ReconcileKey]okErr{
				ritz:     {ok: true},
				triscuit: {ok: true},
			}
			cow := NewCoW(idler)

			By("populating the inactive services")
			errs := exec.PopulateInactiveServices(cow)
			err := utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())

			By("executing the unidling")
			errs = exec.EnsureUnidle(cow)
			err = utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())

			By("verifying the idling status is false")
			Expect(updatedIdler.Status.Idled).To(BeFalse())
		})

		It("should keep idled status at false if some trigger services are missing active endpoints", func() {
			endpointsInfo = map[types.ReconcileKey]okErr{
				ritz:     {ok: true},
				triscuit: {ok: false},
			}
			cow := NewCoW(idler)

			By("populating the inactive services")
			errs := exec.PopulateInactiveServices(cow)
			err := utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())

			By("executing the unidling")
			errs = exec.EnsureUnidle(cow)
			err = utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())

			By("verifying the idling status is still true")
			Expect(updatedIdler.Status.Idled).To(BeTrue())
		})

		It("should restore all target scalables to their original scales, and remove them from the status", func() {
			cow := NewCoW(idler)

			By("populating the inactive services")
			errs := exec.PopulateInactiveServices(cow)
			err := utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())

			By("executing the unidling")
			errs = exec.EnsureUnidle(cow)
			err = utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())

			By("verifying the target scalables are at the desired scales")
			Expect(scales).To(Equal(scalesMap{
				sharpCheddar: replicasOrErr{replicas: 3},
				mildCheddar:  replicasOrErr{replicas: 4},
				american:     replicasOrErr{replicas: 5},
			}))

			By("verifying the previous scales are removed from the status")
			Expect(updatedIdler.Status.UnidledScales).To(BeEmpty())
		})

		It("shouldn't run updates if nothing's changed", func() {
			// set an error so that if we try to update, we'll see an error
			updateError = fmt.Errorf("you shouldn't have come here... 👻")
			endpointsInfo = map[types.ReconcileKey]okErr{
				ritz:     {ok: true},
				triscuit: {ok: true},
			}

			scales[mildCheddar] = replicasOrErr{replicas: 3}
			scales[american] = replicasOrErr{replicas: 4}
			scales[sharpCheddar] = replicasOrErr{replicas: 5}
			idler.Status.UnidledScales = []idling.UnidleInfo{}
			idler.Status.Idled = false
			idler.Status.InactiveServiceNames = []string{}

			cow := NewCoW(idler)

			By("populating the inactive services")
			errs := exec.PopulateInactiveServices(cow)
			err := utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())

			By("executing the unidling")
			errs = exec.EnsureUnidle(cow)
			err = utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Populating Inactive Services", func() {
		BeforeEach(func() {
			idler = &idling.Idler{
				Spec: idling.IdlerSpec{
					TriggerServiceNames: []string{ritz.Name, triscuit.Name},
				},
				Status: idling.IdlerStatus{
					Idled: true,
				},
			}
			updateError = nil
			endpointsInfo = map[types.ReconcileKey]okErr{}
			updatedIdler = nil
		})
		It("shouldn't change anything if the inactive services are the same", func() {
			idler.Status.InactiveServiceNames = []string{triscuit.Name}
			endpointsInfo = map[types.ReconcileKey]okErr{
				ritz:     {ok: true},
				triscuit: {ok: false},
			}

			cow := NewCoW(idler)

			By("populating the inactive services")
			errs := exec.PopulateInactiveServices(cow)
			err := utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())

			By("updating if necessary")
			upErr := cow.UpdateIfNeeded(func(u *idling.Idler) error {
				updatedIdler = u
				return nil
			})
			Expect(upErr).NotTo(HaveOccurred())

			By("verifying that no update was done")
			Expect(updatedIdler).To(BeNil())
		})

		It("should treat failures on fetch the same as inactive", func() {
			endpointsInfo = map[types.ReconcileKey]okErr{
				ritz:     {ok: true},
				triscuit: {err: fmt.Errorf("take off every zig, for great justice")},
			}

			cow := NewCoW(idler)

			By("populating the inactive services")
			errs := exec.PopulateInactiveServices(cow)
			err := utilerrors.NewAggregate(errs)
			Expect(err).To(HaveOccurred())

			By("updating if necessary")
			upErr := cow.UpdateIfNeeded(func(u *idling.Idler) error {
				updatedIdler = u
				return nil
			})
			Expect(upErr).NotTo(HaveOccurred())

			By("verifying that the inactive services list is just the erroring one")
			Expect(updatedIdler.Status.InactiveServiceNames).To(ConsistOf(triscuit.Name))
		})

		It("should update the list when necessary", func() {
			idler.Status.InactiveServiceNames = []string{ritz.Name}
			endpointsInfo = map[types.ReconcileKey]okErr{
				ritz:     {ok: false},
				triscuit: {ok: false},
			}

			cow := NewCoW(idler)

			By("populating the inactive services")
			errs := exec.PopulateInactiveServices(cow)
			err := utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())

			By("updating if necessary")
			upErr := cow.UpdateIfNeeded(func(u *idling.Idler) error {
				updatedIdler = u
				return nil
			})
			Expect(upErr).NotTo(HaveOccurred())

			By("verifying that the inactive services list is correct")
			Expect(updatedIdler.Status.InactiveServiceNames).To(Equal([]string{ritz.Name, triscuit.Name}))

		})

		It("should clear the list when necessary", func() {
			idler.Status.InactiveServiceNames = []string{ritz.Name}
			endpointsInfo = map[types.ReconcileKey]okErr{
				ritz:     {ok: true},
				triscuit: {ok: true},
			}
			cow := NewCoW(idler)

			By("populating the inactive services")
			errs := exec.PopulateInactiveServices(cow)
			err := utilerrors.NewAggregate(errs)
			Expect(err).NotTo(HaveOccurred())

			By("updating if necessary")
			upErr := cow.UpdateIfNeeded(func(u *idling.Idler) error {
				updatedIdler = u
				return nil
			})
			Expect(upErr).NotTo(HaveOccurred())

			By("verifying that the inactive services list is empty")
			Expect(updatedIdler.Status.InactiveServiceNames).To(BeEmpty())
		})
	})
})
