Template service broker
=======================

The template service broker implements an [Open Service Broker
API](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md)
compatible broker which provisions and deprovisions OpenShift
templates.

There are three main components to the work:

1. Template service broker implementation
(/pkg/template/servicebroker).  This plugs into the generic Open
Service Broker API framework mentioned below.  Currently, when
enabled, the template service broker is provided by the OpenShift
master at https://\<master\>:8443/brokers/template.openshift.io/v2/.
The template service broker stores its state in non-namespaced
**BrokerTemplateInstance** objects in etcd.

2. Generic Open Service Broker API and server framework
(/pkg/openservicebroker/{api,server}).  This provides the general
server framework into which individual broker implementations such as
the template service broker can be plugged.

3. TemplateInstance API object and controller
(/pkg/template/controller).  This provides a standard
k8s/OpenShift-style mechanism to instantiate templates, which is
consumed by the template service broker and may in the future have
additional consumers.  The **TemplateInstance** controller stores its
state in namespaced **TemplateInstance** objects in etcd.

TemplateInstance API object and controller
------------------------------------------

A **TemplateInstance** API object *Spec* contains a full copy of a
**Template**, a reference to a **Secret**, and the identity of a user.

When a **TemplateInstance** API object is created in a particular
namespace, the **TemplateInstance** controller will instantiate the
template according to the parameters contained in the referred
**Secret**, using the user's privileges.

All objects created by the **TemplateInstance** controller are
labelled with reference to the **TemplateInstance** object, and the
**TemplateInstance** object is also added to created objects'
*OwnerReferences*.  This has the effect that when a
**TemplateInstance** object is deleted, the garbage collector should
automatically remove all objects associated to the
**TemplateInstance** object which were created by the controller.

Currently, **TemplateInstance** objects are effectively immutable once
created.

Template service broker
-----------------------

The Template service broker implements the [Open Service Broker
API](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md)
endpoints:

- *Catalog*: returns a list of available templates as OSB API
  *Service* objects (the templates are read from one or more
  namespaces configured in the master config).

- *Provision*: provision a given template (referred by its UID) into a
  namespace.  Under the covers, this creates a non-namespaced
  **BrokerTemplateInstance** object for the template service broker to
  store state associated with the the instantiation, as well as the
  **Secret** and **TemplateInstance** objects which are picked up by
  the **TemplateInstance** controller.  *Provision* is an asynchronous
  operation: it may return before provisioning is completed, and the
  provision status can (must) be recovered via the *Last Operation*
  endpoint (see below).

- *Bind*: for a given template, return "credentials" exposed in any
  created ConfigMap, Secret, Service or Route object (see
  ExposeAnnotationPrefix and Base64ExposeAnnotationPrefix
  documentation).  The *Bind* call records the fact that it took
  place in the appropriate **BrokerTemplateInstance** object.

- *Unbind*: this simply removes the metadata previously placed in the
  **BrokerTemplateInstance** object by a *Bind* call.

- *Deprovision*: removes the objects created by the *Provision* call.
  The garbage collector removes all additional objects created by the
  **TemplateInstance** controller, hopefully transitively, as
  documented above.

- *Last Operation*: returns the status of the previously run
  asynchronous operation.  In the template service broker, *Provision*
  is the only asynchronous operation.

The template service broker is enabled by adding the following
(example) configuration to the OpenShift master config and restarting
the master:

    templateServiceBrokerConfig:
      templateNamespaces:
      - openshift

When enabled, the template service broker is currently provided by the
OpenShift master at
https://\<master\>:8443/brokers/template.openshift.io/v2/.

Simple shell scripts which use `curl` to query the API can be found in
the test-scripts/ subdirectory.  See the README.md file contained
therein for more details.
