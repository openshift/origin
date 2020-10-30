all: pubkeys ca leaves

# The following private keys are never regenerated.
SERVER_PRIVKEYS=ct-http-server.privkey.pem log-rpc-server.privkey.pem

# Corresponding passwords:
CT_HTTP_PWD=dirk
LOG_RPC_PWD=towel
MAP_RPC_PWD=towel

# Server public keys are derived from the corresponding private keys.
SERVER_PUBKEYS=$(subst .privkey,.pubkey,$(SERVER_PRIVKEYS))

# Build public keys from private keys
pubkeys: $(SERVER_PUBKEYS)
log-rpc-server.pubkey.pem: log-rpc-server.privkey.pem
	openssl ec -in $< -pubout -out $@ -passin pass:$(LOG_RPC_PWD)
ct-http-server.pubkey.pem: ct-http-server.privkey.pem
	openssl ec -in $< -pubout -out $@ -passin pass:$(CT_HTTP_PWD)


# We use a fake CA as a trust root for CT tests. This is its private key.
CA_PRIVKEY=fake-ca.privkey.pem
CA_PWD=gently

# We also have an intermediate CA, with private key:
INT_CA_PRIVKEY=int-ca.privkey.pem
INT_CA_PWD=babelfish

# All the leaf certificates share a private key:
LEAF_PRIVKEY=leaf.privkey.pem
LEAF_PWD=liff

ca: fake-ca.cert int-ca.cert

# Fake Root CA
fake-ca.cert: $(CA_PRIVKEY) fake-ca.cfg
	openssl req -new -x509 -config fake-ca.cfg -set_serial 0x0406cafe -days 3650 -extensions v3_ca -inform pem -key $(CA_PRIVKEY) -passin pass:$(CA_PWD) -out $@
show-ca: fake-ca.cert
	openssl x509 -inform pem -in $< -text -noout

# Fake Intermediate CA
int-ca.csr.pem: $(INT_CA_PRIVKEY) int-ca.cfg
	openssl req -new -sha256 -config int-ca.cfg -key $(INT_CA_PRIVKEY) -passin pass:$(INT_CA_PWD) -out $@
show-int-csr: int-ca.csr.pem
	openssl req -in $< -text -noout
int-ca.cert: int-ca.csr.pem $(CA_PRIVKEY) fake-ca.cert
	openssl x509 -req -in int-ca.csr.pem -sha256 -extfile fake-ca.cfg -extensions v3_int_ca -CA fake-ca.cert -CAkey $(CA_PRIVKEY) -passin pass:$(CA_PWD) -set_serial 0x42424242 -days 3600 -out $@
show-int-ca: int-ca.cert
	openssl x509 -inform pem -in $< -text -noout

# Leaf Certificates
LEAF_CERTS=leaf01.cert leaf02.cert leaf03.cert leaf04.cert leaf05.cert leaf06.cert leaf07.cert leaf08.cert leaf09.cert leaf10.cert \
	leaf11.cert leaf12.cert leaf13.cert leaf14.cert leaf15.cert leaf16.cert leaf17.cert leaf18.cert leaf19.cert leaf20.cert
LEAF_CSRS=$(subst .cert,.csr.pem,$(LEAF_CERTS))
LEAF_CHAINS=$(subst .cert,.chain,$(LEAF_CERTS))
leaves: $(LEAF_CERTS) $(LEAF_CHAINS)

leaf%.csr.pem: $(LEAF_PRIVKEY)
	openssl req -new -sha256 -key $(LEAF_PRIVKEY) -passin pass:$(LEAF_PWD) -subj "/C=GB/ST=London/O=Google/OU=Eng/CN=$@" -out $@
show-leaf%-csr: leaf%.csr.pem
	openssl req -in $< -text -noout
leaf%.cert: leaf%.csr.pem int-ca.cert
	openssl x509 -req -in $< -sha256 -extfile int-ca.cfg -extensions v3_user -CA int-ca.cert -CAkey $(INT_CA_PRIVKEY) -passin pass:$(INT_CA_PWD) -set_serial 0xdeadbeef -days 2600 -out $@
show-leaf%: leaf%.cert
	openssl x509 -inform pem -in $< -text -noout
leaf%.chain: leaf%.cert int-ca.cert
	cat $^ > $@
# Special case: include the root too
leaf02.chain: leaf02.cert int-ca.cert fake-ca.cert
	cat $^ > $@
# Special case: add an unknown EKU
leaf03.cert: leaf03.csr.pem leaf.privkey.pem int-ca.cert
	openssl x509 -req -in $< -sha256 -extfile int-ca.cfg -extensions v3_user_plus -CA int-ca.cert -CAkey int-ca.privkey.pem  -passin pass:$(INT_CA_PWD) -set_serial 0xdeadbeef -days 2600 -out $@


# Pair of intermediate CAs for a longer chain
INT_CA_1_PRIVKEY=int-ca-1.privkey.pem
INT_CA_2_PRIVKEY=int-ca-2.privkey.pem
INT_CA_1_PWD=vogon
INT_CA_2_PWD=vogon

# Fake Intermediate CA 1
int-ca-1.csr.pem: $(INT_CA_PRIVKEY) int-ca.cfg
	openssl req -new -sha256 -config int-ca.cfg -key $(INT_CA_1_PRIVKEY) -passin pass:$(INT_CA_1_PWD) -out $@
int-ca-1.cert: int-ca-1.csr.pem $(CA_PRIVKEY) fake-ca.cert
	openssl x509 -req -in int-ca-1.csr.pem -sha256 -extfile fake-ca.cfg -extensions v3_int_ca_pair -CA fake-ca.cert -CAkey $(CA_PRIVKEY) -passin pass:$(CA_PWD) -set_serial 0x01010101 -days 3600 -out $@

# Fake 2nd-level Intermediate CA 2
int-ca-2.csr.pem: int-ca-2.privkey.pem int-ca-2.cfg
	openssl req -new -sha256 -config int-ca-2.cfg -key $(INT_CA_2_PRIVKEY) -passin pass:$(INT_CA_2_PWD) -out $@
int-ca-2.cert: int-ca-2.csr.pem $(INT_CA_PRIVKEY) int-ca-1.cert
	openssl x509 -req -in int-ca-2.csr.pem -sha256 -extfile fake-ca.cfg -extensions v3_int_ca_pair -CA int-ca-1.cert -CAkey $(INT_CA_1_PRIVKEY) -passin pass:$(INT_CA_1_PWD) -set_serial 0x12121212 -days 3600 -out $@

# Length 4 chain (to allow mis-ordering tests).
subleaf.csr.pem: $(LEAF_PRIVKEY)
	openssl req -new -sha256 -key $(LEAF_PRIVKEY) -passin pass:$(LEAF_PWD) -subj "/C=GB/ST=London/O=Google/OU=Eng/CN=$@" -out $@
subleaf.cert: subleaf.csr.pem int-ca-2.cert
	openssl x509 -req -in $< -sha256 -extfile int-ca-2.cfg -extensions v3_user -CA int-ca-2.cert -CAkey $(INT_CA_2_PRIVKEY) -passin pass:$(INT_CA_2_PWD) -set_serial 0xdeadbeef -days 2600 -out $@
subleaf.chain: subleaf.cert int-ca-2.cert int-ca-1.cert fake-ca.cert
	cat $^ > $@
subleaf.misordered.chain: subleaf.cert int-ca-1.cert int-ca-2.cert fake-ca.cert
	cat $^ > $@


# clean removes things that regenerate exactly the same.
clean:
	rm -f $(SERVER_PUBKEYS)
# distclean removes things that regenerate with changes (e.g. timestamped, randomized).
distclean: clean
	rm -f $(SERVER_PUBKEYS) fake-ca.cert int-ca.cert int-ca.csr.pem
	rm -f $(LEAF_CERTS) $(LEAF_CSRS) $(LEAF_CHAINS)

# The newkey target creates a fresh private key; should never be needed.
newkey: fresh.privkey.pem
fresh.privkey.pem:
	openssl ecparam -genkey -name prime256v1 -noout -out $@.unencrypted
	openssl ec -in $@.unencrypted -out $@ -des  # Prompts for password
	rm -f $@.unencrypted
