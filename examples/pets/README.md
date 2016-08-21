# PetSet examples

These examples are tracked from the [Kubernetes contrib project @d6e4be](https://github.com/kubernetes/contrib/tree/d6e4be066cc076fbb91ff69691819e117711b30b/pets)

Note that some of these examples require the ability to run root containers which may not be possible for all users in all environments. To grant
access to run containers as root to a service account in your project, run:

    oadm policy add-scc-to-user anyuid -z default

which allows the `default` service account to run root containers.