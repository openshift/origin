In terminal 1:
```
sudo ./crio
```

In terminal 2:
```
sudo ./crictl runtimeversion

sudo rm -rf /var/lib/containers/storage/sandboxes/podsandbox1
sudo ./crictl runs testdata/sandbox_config.json

sudo rm -rf /var/lib/containers/storage/containers/container1
sudo ./crictl create podsandbox1 testdata/container_config.json testdata/sandbox_config.json
```
