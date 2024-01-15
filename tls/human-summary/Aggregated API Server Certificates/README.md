# Aggregated API Server Certificates

Aggregated API Server Certificates

![PKI Graph](cert-flow.png)

- [Signing Certificate/Key Pairs](#signing-certificatekey-pairs)
    - [aggregator-signer](#aggregator-signer)
- [Serving Certificate/Key Pairs](#serving-certificatekey-pairs)
- [Client Certificate/Key Pairs](#client-certificatekey-pairs)
    - [system:openshift-aggregator](#systemopenshift-aggregator)
- [Certificates Without Keys](#certificates-without-keys)
- [Certificate Authority Bundles](#certificate-authority-bundles)
    - [aggregator-signer](#aggregator-signer)

## Signing Certificate/Key Pairs


### aggregator-signer
![PKI Graph](subcert-aggregator-signer5053783812448525690.png)



| Property | Value |
| ----------- | ----------- |
| Type | Signer |
| CommonName | aggregator-signer |
| SerialNumber | 5053783812448525690 |
| Issuer CommonName | [aggregator-signer](#aggregator-signer) |
| Validity | 24h |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment<br/>- KeyUsageCertSign |
| ExtendedUsages |  |


#### aggregator-signer Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver-operator | aggregator-client-signer |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



## Serving Certificate/Key Pairs

## Client Certificate/Key Pairs


### system:openshift-aggregator
![PKI Graph](subcert-systemopenshift-aggregator2219035947576355460.png)



| Property | Value |
| ----------- | ----------- |
| Type | Client |
| CommonName | system:openshift-aggregator |
| SerialNumber | 2219035947576355460 |
| Issuer CommonName | [aggregator-signer](#aggregator-signer) |
| Validity | 12h |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageClientAuth |
| Organizations (User Groups) |  |


#### system:openshift-aggregator Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver | aggregator-client |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



## Certificates Without Keys

These certificates are present in certificate authority bundles, but do not have keys in the cluster.
This happens when the installer bootstrap clusters with a set of certificate/key pairs that are deleted during the
installation process.

## Certificate Authority Bundles


### aggregator-signer
![PKI Graph](subca-668341161.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [aggregator-signer](#aggregator-signer) | [aggregator-signer](#aggregator-signer) | 24h | RSA 2048 bit |

#### aggregator-signer Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-config-managed | kube-apiserver-aggregator-client-ca |
| openshift-kube-apiserver | aggregator-client-ca |
| openshift-kube-controller-manager | aggregator-client-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



