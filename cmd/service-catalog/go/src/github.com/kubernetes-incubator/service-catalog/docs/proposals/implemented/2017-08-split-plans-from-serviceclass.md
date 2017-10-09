# Split Plans from ServiceClasses

Proposed: 2017-08-08
Approved: 2017-08-15

## Abstract

Proposal to split Plans from ServiceClass to become a top-level object in our API.

## Motivation

If we get a ServiceClass, we get all of the Plans, which potentially contain a
lot of data. The data of concern is primarily the Schema information used for
exposing the details of a ServicePlan to a UI.

## Constraints and Assumptions

 - It is desired to have the ServicePlan data in one place and not
   duplicated. Duplicated data gets out of sync.

## Proposed Design

 - new ServicePlan resource based on existing ServicePlan object
 - Implement FieldSelector for ServicePlan on `Free` field combined with ServiceClass Reference

### ServiceClass API Changes

We remove the array of ServicePlans defined inline.

``` go
// ServiceClass represents an offering in the service catalog.
type ServiceClass struct {
    ...

    // All references to ServicePlans are removed
    
    ...
}
```

### ServicePlan API Resource

Added the kubernetes TypeMeta and ObjectMeta.

Add a Kubernetes LocalObjectReference for the ServiceClass that owns
the ServicePlan.

Annotations for client and not being namespaced.

No other changes.

```go
// ServicePlan represents a tier of a ServiceClass.
type ServicePlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

    ...
    
        // ServiceClass is the Class of Service that this plan implements. 
	ServiceClass v1.LocalObjectReference `json:"serviceClass"`
}
```

```
// ServicePlanList is a list of instances.
type ServicePlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ServicePlan `json:"items"`
}
```

ServiceClass is cluster-scoped, so ServicePlan is cluster-scoped. Possibility
of collision of two ServicePlans from different ServiceClass based on
Name. This will be addressed by concatinating the ServiceClass
ExternalID onto the end of the incoming ServicePlan Name and separated
by a dash.

### API Server Changes

Addition of new types. A not namespaced type ServicePlan and the list of the
ServicePlan type. Plumbing of ServicePlan from rest input to storage. Validation
code changes for ServiceClass and ServicePlan. 

### Controller-Manager Changes

Creation of ServiceClass separate from ServicePlan. 

Separate creation of each individual ServicePlan.

Create each ServicePlan with link to the owning ServiceClass. Create
ServiceClass after all ServicePlans have been created.

Lookup of a ServicePlan changes. Lookup by name can be done directly.

Lookup of all Plans a serviceclass owns involves iterating over all of
the plans from every service class and accumulating them. 

### Admission-Controllers

ServicePlanDefault controller is modified to adapt to new API and client.

### Build Changes

No changes, but new build output. Code generation output for ServicePlan. Client
and listers and informers.

## Drawbacks

 - Inefficient to find ServicePlans owned by a ServiceClass. Fixed by
   implementing the field selector on plans and using a field
   selection on the ServiceClassRef field.

## Later Options
 - Create a Plan stub object consisting of the free data and pointer to Plan.
 - Modify ServiceClass to have Status of number of free and non-free plans
   filled in by controller.
 - Implementation of FieldSelector on ServicePlan for the Free and
   ServiceClass Reference fields. #1124 can be done at same time, or
   after this.
