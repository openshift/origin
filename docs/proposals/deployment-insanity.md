Dropping into a doc so we have a place to argue.

We want to enabling deployments(Ds) and someday we want feature parity with DCs and then we'll likely retire DCs.

This means that we need to make sure that no D or DC have conflicting names.  The simplest way to 
do this is to have the two resources cohabitate (live in the same directory in etcd).  However,
they are different than our other migrating/cohabitating resources like replicationcontrollers and 
replicasets.  RC and RS controllers are close enough to parity that we're going to be able to run
a single controller to manage both.  D and DC controllers are not close enough to do that.

Since we'll be running two controllers, we'll need to have the controllers know which resources under
the D or DC endpoint they should be managing.  We don't yet have controllerRefs and we're unlikely to 
pick them in the next couple days.  The fastest way that I see to do this is:
 1. Extend `StorageFactory` to allow the storage encoding version for cohabitating resources vary based
 on which resource is being requested.
 2. Update conversions from `DC.v1->D.__internal.extensions` and `D.v1beta1.extensions->DC.__internal` to
 add an annotation: `k8s.io/original-kind=GroupKind`.
 3. Add full-fidelity conversions between the types.
 4. If a D receives an Update with `k8s.io/original-kind=DC.v1`, it could convert to a DC 
 (which should remove the annotation) and find a way to call the DC storage.

As an alternative to 3 and 4, we could
 3. Not add full-fidelity conversions.
 4. If a D receives an Update with `k8s.io/original-kind=DC.v1`, it could just fail.

The second alternative is easier to get done and I think it keeps the web console mostly working.
You'd be able to see Ds and we can probably wire up D/scale and DC/scale to special case each other's
resources so that basic scaling would work.  You'll hit weirdness with bulk labelling and annotating,
but there's nothing stopping us from making it cleaner in the future.