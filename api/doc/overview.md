The 3.x model attempts to expose underlying Docker and Google models as
accurately as possible, with a focus on easy composition of applications
by a developer (install Ruby, push code, add MySQL). Unlike 2.x, more
flexibility of configuration is exposed after creation in all aspects
of the model. Terminology is still being weighed, but the concept of an
application as a separate object is being removed in favor of more flexible
composition of "services" - allowing two web containers to reuse a DB,
or expose a DB directly to the edge of the network.
The existing API will continue to be supported through 3.x,
with concepts mapped as closely as possible to the new model.
