# Unknown Networking 02

Unknown Networking 02

![PKI Graph](cert-flow.png)

- [Signing Certificate/Key Pairs](#signing-certificatekey-pairs)
    - [openshift-network-node-identity_network-node-identity-ca@1704273764](#openshift-network-node-identity_network-node-identity-ca1704273764)
- [Serving Certificate/Key Pairs](#serving-certificatekey-pairs)
    - [127.0.0.1](#127.0.0.1)
- [Client Certificate/Key Pairs](#client-certificatekey-pairs)
    - [127.0.0.1](#127.0.0.1)
- [Certificates Without Keys](#certificates-without-keys)
- [Certificate Authority Bundles](#certificate-authority-bundles)
    - [openshift-network-node-identity_network-node-identity-ca@1704273764](#openshift-network-node-identity_network-node-identity-ca1704273764)

## Signing Certificate/Key Pairs


### openshift-network-node-identity_network-node-identity-ca@1704273764
![PKI Graph](subcert-openshift-network-node-identity_network-node-identity-ca17042737641372784190514149555.png)



| Property | Value |
| ----------- | ----------- |
| Type | Signer |
| CommonName | openshift-network-node-identity_network-node-identity-ca@1704273764 |
| SerialNumber | 1372784190514149555 |
| Issuer CommonName | [openshift-network-node-identity_network-node-identity-ca@1704273764](#openshift-network-node-identity_network-node-identity-ca1704273764) |
| Validity | 10y |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment<br/>- KeyUsageCertSign |
| ExtendedUsages |  |


#### openshift-network-node-identity_network-node-identity-ca@1704273764 Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-network-node-identity | network-node-identity-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



## Serving Certificate/Key Pairs


### 127.0.0.1
![PKI Graph](subcert-127.0.0.1103972737128662425.png)



| Property | Value |
| ----------- | ----------- |
| Type | Serving,Client |
| CommonName | 127.0.0.1 |
| SerialNumber | 103972737128662425 |
| Issuer CommonName | [openshift-network-node-identity_network-node-identity-ca@1704273764](#openshift-network-node-identity_network-node-identity-ca1704273764) |
| Validity | 182d |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageServerAuth<br/>- ExtKeyUsageClientAuth |
| Organizations (User Groups) |  |
| DNS Names | - 127.0.0.1 |
| IP Addresses | - 127.0.0.1 |


#### 127.0.0.1 Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-network-node-identity | network-node-identity-cert |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



## Client Certificate/Key Pairs


### 127.0.0.1
![PKI Graph](subcert-127.0.0.1103972737128662425.png)



| Property | Value |
| ----------- | ----------- |
| Type | Serving,Client |
| CommonName | 127.0.0.1 |
| SerialNumber | 103972737128662425 |
| Issuer CommonName | [openshift-network-node-identity_network-node-identity-ca@1704273764](#openshift-network-node-identity_network-node-identity-ca1704273764) |
| Validity | 182d |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageServerAuth<br/>- ExtKeyUsageClientAuth |
| Organizations (User Groups) |  |
| DNS Names | - 127.0.0.1 |
| IP Addresses | - 127.0.0.1 |


#### 127.0.0.1 Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-network-node-identity | network-node-identity-cert |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



## Certificates Without Keys

These certificates are present in certificate authority bundles, but do not have keys in the cluster.
This happens when the installer bootstrap clusters with a set of certificate/key pairs that are deleted during the
installation process.

## Certificate Authority Bundles


### openshift-network-node-identity_network-node-identity-ca@1704273764
![PKI Graph](subca-3127336931.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [openshift-network-node-identity_network-node-identity-ca@1704273764](#openshift-network-node-identity_network-node-identity-ca1704273764) | [openshift-network-node-identity_network-node-identity-ca@1704273764](#openshift-network-node-identity_network-node-identity-ca1704273764) | 10y | RSA 2048 bit |

#### openshift-network-node-identity_network-node-identity-ca@1704273764 Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-network-node-identity | network-node-identity-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



