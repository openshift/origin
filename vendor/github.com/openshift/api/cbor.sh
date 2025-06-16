#!/bin/bash

# In OpenShift, etcd pods are typically named differently and may require different authentication
ETCD_POD=$(oc get pods -n openshift-etcd -l k8s-app=etcd -o jsonpath='{.items[0].metadata.name}')

# Function to check if a value is CBOR (starts with byte 0xCF)
is_cbor() {
  local value="$1"
  # echo "$value" | hexdump -C | head -n 1
  echo -n "$value" | hexdump -C | head -n 1 | grep -q "^00000000  d9 d9 f7"
  return $?
}

echo "Using etcd pod: $ETCD_POD"

# Get all keys from etcd
# Note: In OpenShift, we need to use the proper certificates and endpoints
echo "Getting all keys from etcd..."
KEYS=$(oc exec -n openshift-etcd $ETCD_POD -- etcdctl get --prefix --keys-only /kubernetes.io/operator.openshift.io)

# Check each key
echo "Checking all keys for CBOR encoding..."
echo "CBOR-encoded objects:"
echo "---------------------"

count=0
for key in $KEYS; do
  # Skip empty lines
  if [ -z "$key" ]; then
    continue
  fi
  
  # Get the value for the key
  # value=$(oc exec -n openshift-etcd $ETCD_POD -- etcdctl get "$key" --print-value-only 2>/dev/null)
  value=$(oc exec -n openshift-etcd "$ETCD_POD" -- etcdctl get "$key" --print-value-only 2>/dev/null | tr -d '\0')

  
  # Check if the value is CBOR
  if is_cbor "$value"; then
    echo "$key"
    count=$((count+1))
  fi
done

echo "---------------------"
echo "Found $count CBOR-encoded objects in etcd"
