TEST_IMAGE=atomic-registry-quickstart
INSTALLHOST=127.0.0.1

all: build install test

build:
	docker build -t atomic-registry-quickstart .

install:
	sudo yum install -y atomic
	sudo atomic install $(TEST_IMAGE) $(INSTALLHOST)
	# FIXME: docker error: installer container is not removed using --rm
	sudo atomic run $(TEST_IMAGE) $(INSTALLHOST)

test:
	bash test.sh 127.0.0.1

clean:
	sudo atomic uninstall --force $(TEST_IMAGE)
	sudo docker rm $(sudo docker ps -qa)
