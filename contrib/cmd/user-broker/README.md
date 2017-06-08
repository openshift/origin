# Service Broker

This is an implementation of a service broker which implements User Provided
Service Instances.  It runs within the context of a Service Controller and does
not reify any resources. It only hangs onto the binding information that is
passed on during creation of User Provided Service Instance and returns it upon
binding to this service.
