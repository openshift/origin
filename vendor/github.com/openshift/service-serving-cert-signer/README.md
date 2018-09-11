# service-serving-cert-signer
Coming soon...

Controller to mint and manage serving certificates for Kubernetes services.

Current thinking of how this will be structured (this is subject to change).
 1. Controller to create service serving cert/key pairs as today
 2. Controller to keep apiservices up to date with a CA bundle based on an annotation on the apiservice.
 3. Controller to maintain (but not create) configmaps with a CA bundle.
 4. Operator to manage the three controllers and keep the CA bundle used to try 2 and 3 up to date with the service serving cert CA.
