# TestSmokeTest
This functional test can be used on a system with at least 8GB of RAM.

## Requirements

* Vagrant
* Ansible
* Hypervisor: VirtualBox or Libvirt/KVM

## Create VMs and run all the Tests

 Just execute `run.sh` in this directory `tests/functional/TestSmokeTest`. It does the following:
* creates 4 VMs using vagrant
* builds heketi
* starts heketi
* executes every test in `./tests/heketi_test.go`
* stops heketi
* deletes heketi.db
* destroys all 4 VMs

```bash
$ ./run.sh
```

Output will be shown by the logs on the heketi server.

### To retain VMs after test
If you wish to retain the setup after the tests, you can use the following command instead:

```bash
$ HEKETI_TEST_CLEANUP="no" ./run.sh
```

Heketi server is stopped but heketi.db and VMs are retained.

### To reuse VMs that are retained
If you wish to reuse the setup after the tests, use the following command:

```bash
$ HEKETI_TEST_VAGRANT="no" ./run.sh
```

`HEKETI_TEST_VAGRANT="no"` skips all vagrant operations but the heketi.db file is deleted. Combine with `HEKETI_TEST_CLEANUP="no"` to retain heketi.db.



## To create VMs

* Go to `tests/functional/TestSmokeTest/vagrant`
Type:
```
$ ./up.sh --provider=PROVIDER
```
where PROVIDER is virtualbox or libvirt.

## Running the Tests

* Go to the top of the source tree build and run make

```
$ make
$ cd tests/functional/TestSmokeTest
$ rm heketi.db
$ cp ../../../heketi ../heketi
$ ../heketi --config=./config/heketi.json

```

* Once it is ready, then start running the tests in another terminal

```
$ cd tests/functional/TestSmokeTest/tests
$ go test -tags functional
```

Output will be shown by the logs on the heketi server.

### To run individual tests
```
$ cd tests/functional/TestSmokeTest/tests
$ go test -tags functional -run TestName
```

### To use VMs with different IPs
The tests use IPs of VMs created in `up.sh` by default. If you wish to provide different machines use the ENV variables `HEKETI_TEST_STORAGE{0..3}`. For example:
```
$ cd tests/functional/TestSmokeTest/tests
$ HEKETI_TEST_STORAGE0="192.168.10.200" HEKETI_TEST_STORAGE1="192.168.10.201" HEKETI_TEST_STORAGE2="192.168.10.202" HEKETI_TEST_STORAGE3="192.168.10.203" go test -tags functional -run TestRemoveDevice
```
