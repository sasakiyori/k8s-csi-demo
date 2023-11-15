.PHONY: increment_version deploy

BIN = k8s-csi-demo

VERSION_FILE = Version
VERSION := $(shell cat $(VERSION_FILE))
MAJOR := $(word 1,$(subst ., ,$(VERSION)))
MINOR := $(word 2,$(subst ., ,$(VERSION)))
PATCH := $(word 3,$(subst ., ,$(VERSION)))

build:
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ${BIN} main.go

deploy: increment_version
	@docker build -t ${BIN}:${VERSION} ./

clean:
	@rm -f ${BIN}

increment_version:
	$(eval PATCH := $(shell echo $$(($(PATCH)+1))))
	$(eval VERSION := $(MAJOR).$(MINOR).$(PATCH))
	@echo "$(VERSION)" > $(VERSION_FILE)
