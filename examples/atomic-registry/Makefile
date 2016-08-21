TEST_IMAGE=atomic-registry-install
INSTALLHOST=127.0.0.1

all-systemd: pull-images build-systemd install-atomic install-systemd test-systemd
all-allinone: pull-images build-allinone install-atomic install-allinone test-allinone

pull-images:
	for IMAGE in openshift/origin openshift/origin-docker-registry cockpit/kubernetes ; do \
			docker pull $$IMAGE ; \
	done

build-systemd:
	docker build -t $(TEST_IMAGE) systemd/.

build-allinone:
	docker build -t $(TEST_IMAGE) allinone/.

install-atomic:
	dnf install -y atomic

install-systemd:
	atomic install $(TEST_IMAGE) $(INSTALLHOST)
	systemctl enable --now atomic-registry-master.service
	sleep 10
	/var/run/setup-atomic-registry.sh $(INSTALLHOST)

install-allinone:
	atomic install $(TEST_IMAGE) $(INSTALLHOST)
	atomic run $(TEST_IMAGE) $(INSTALLHOST)

test-systemd:
	bash test.sh atomic-registry-master 127.0.0.1

test-allinone:
	bash test.sh origin 127.0.0.1

clean:
	atomic uninstall $(TEST_IMAGE) --remove-data
	docker rm `docker ps -qa`
