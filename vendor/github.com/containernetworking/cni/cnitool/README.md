# cnitool

`cnitool` is a simple program that executes a CNI configuration. It will
add or remove an interface in an already-created network namespace.

## Environment Variables

* `NETCONFPATH`: This environment variable needs to be set to a
  directory. It defaults to `/etc/cni/net.d`. The `cnitool` searches
  for CNI configuration files in this directory with the extension
  `*.conf` or `*.json`. It loads all the CNI configuration files in
  this directory and if it finds a CNI configuration with the `network
  name` given to the cnitool it returns the corresponding CNI
  configuration, else it returns `nil`.
* `CNI_PATH`: For a given CNI configuration `cnitool` will search for
  the corresponding CNI plugin in this path.

## Example invocation

First, install cnitool:

```bash
go get github.com/containernetworking/cni
go install github.com/containernetworking/cni/cnitool
```

Then, check out and build the plugins. All commands should be run from this directory.

```bash
git clone https://github.com/containernetworking/plugins.git
cd plugins
./build_linux.sh
# or
./build_windows.sh
```

Create a network configuration

```bash
echo '{"cniVersion":"0.4.0","name":"myptp","type":"ptp","ipMasq":true,"ipam":{"type":"host-local","subnet":"172.16.29.0/24","routes":[{"dst":"0.0.0.0/0"}]}}' | sudo tee /etc/cni/net.d/10-myptp.conf
```

Create a network namespace. This will be called `testing`:

```bash
sudo ip netns add testing
```

Add the container to the network:

```bash
sudo CNI_PATH=./bin cnitool add myptp /var/run/netns/testing
```

Check whether the container's networking is as expected (ONLY for spec v0.4.0+):

```bash
sudo CNI_PATH=./bin cnitool check myptp /var/run/netns/testing
```

Test that it works:

```bash
sudo ip -n testing addr
sudo ip netns exec testing ping -c 1 4.2.2.2
```

And clean up:

```bash
sudo CNI_PATH=./bin cnitool del myptp /var/run/netns/testing
sudo ip netns del testing
```
