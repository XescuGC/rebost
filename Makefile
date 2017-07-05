GO_BIN = $(shell which go)
PROJ_DIR := $(shell pwd)
VERBOSE := 1

DOCKER_TEST = docker run --rm rebost-test go test ./...

# HACK: This is a trick because when deploying the GO_BIN is undefined (empty)
# so we harcode it to the default Debian installation to be able to use it
ifeq ($(GO_BIN),)
	GO_BIN = /usr/local/go/bin/go
endif

serve:
	gin -p 8000 -a 8001 -b rebost

deps:
	$(GO_BIN) get -u github.com/boltdb/bolt/... \
									 github.com/gorilla/mux \
									 github.com/satori/go.uuid \
									 github.com/shirou/gopsutil

devDeps:
	$(GO_BIN) get -u github.com/codegangsta/gin

build:
	docker build -t rebost -f Dockerfile.build .

start:
	docker run -ti --rm -p 8000:8000 -v $(PROJ_DIR):/go/src/github.com/xescugc/rebost/ rebost make serve

buildTest:
	docker build -t rebost-test -f Dockerfile.test .

test: buildTest
ifeq ($(VERBOSE), 0)
	$(DOCKER_TEST)
else
	$(DOCKER_TEST) -v
endif
