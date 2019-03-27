all: ca

# The following private keys are never regenerated.
SERVER_PRIVKEYS=gossiper.privkey.pem

# Server public keys are derived from the corresponding private keys.
SERVER_PUBKEYS=$(subst .privkey,.pubkey,$(SERVER_PRIVKEYS))

# Build public keys from private keys
pubkeys: $(SERVER_PUBKEYS)
gossiper.pubkey.pem: gossiper.privkey.pem
	openssl ec -in $< -pubout -out $@ -passin pass:$(GOSSIPER_PWD)

ROOT_CA_PRIVKEY=gossiper.privkey.pem
ROOT_CA_PWD=hissing-sid

ca: root-ca.cert

# Fake Root CA
root-ca.cert: gossiper.privkey.pem root-ca.cfg
	openssl req -new -x509 -config root-ca.cfg -set_serial 0x0406cafe -days 3650 -extensions v3_ca -inform pem -key gossiper.privkey.pem -passin pass:$(ROOT_CA_PWD) -out $@
show-ca: root-ca.cert
	openssl x509 -inform pem -in $< -text -noout

# clean removes things that regenerate exactly the same.
clean:
	rm -f $(SERVER_PUBKEYS)
# distclean removes things that regenerate with changes (e.g. timestamped, randomized).
distclean: clean
	rm -f $(SERVER_PUBKEYS) root-ca.cert

# The newkey target creates a fresh private key; should never be needed.
newkey: fresh.privkey.pem
fresh.privkey.pem:
	openssl ecparam -genkey -name prime256v1 -noout -out $@.unencrypted
	openssl ec -in $@.unencrypted -out $@ -des  # Prompts for password
	rm -f $@.unencrypted
