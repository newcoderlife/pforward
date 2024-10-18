VERSION := $(shell git rev-parse --short HEAD)
ARCHITECTURES := amd64 arm64
TAG ?= $(shell git describe --tags --abbrev=0)
SAVE_ARCH ?= linux/arm64

all: prepare-docker test

.PHONY: force-push
force-push:
	@git add . && git commit --amend --no-edit && git push -f && git tag $(TAG) -f && git push origin -f $(TAG);

.PHONY: go-version
go-version:
	@cat ./tmp/coredns/.go-version

.PHONY: install-dependency
install-dependency:
	@sudo apt-get install debhelper base-files dpkg-dev jq fakeroot dnsutils -y

.PHONY: clone-dependency
clone-dependency:
	@rm -rf tmp && mkdir tmp
	@git clone --depth 1 https://github.com/coredns/coredns.git tmp/coredns

.PHONY: build-coredns
build-coredns: install-dependency clone-dependency
	@echo "pforward:github.com/newcoderlife/pforward" >> ./tmp/coredns/plugin.cfg
	@cat ./tmp/coredns/plugin.cfg
	@$(MAKE) -C ./tmp/coredns/ gen
	@cd ./tmp/coredns && go get "github.com/newcoderlife/pforward@$(VERSION)"
	@$(MAKE) -C ./tmp/coredns/ -f Makefile.release build LINUX_ARCH="amd64 arm64"

.PHONY: pack-coredns
pack-coredns: build-coredns
	rm -rf ../*.deb
	for arch in $(ARCHITECTURES); do \
		VERSION=$(TAG) SRC=./tmp/coredns BIN=./tmp/coredns/build/linux/$$arch/coredns dpkg-buildpackage -us -uc -b -a"$$arch"; \
	done

.PHONY: prepare-docker
prepare-docker: pack-coredns
	rm -rf *.deb
	for arch in $(ARCHITECTURES); do \
		DEB=$$(find ../ -name "*$${arch}.deb"); \
		mv $${DEB} ./coredns_$${arch}.deb; \
	done
	ls

.PHONY: test
test:
	@docker stop test-dns && docker rm test-dns || true
	@docker container prune -f && docker image prune -af || true
	@docker build --no-cache -t ghcr.io/newcoderlife/coredns:latest .
	@image_id=$$(docker images ghcr.io/newcoderlife/coredns --format "{{.ID}}" | head -n 1); \
	docker run -d --name test-dns -p 5353:53/udp $${image_id}; \
	sleep 30
	result=$$(dig @127.0.0.1 -p 5353 one.one.one.one +short | head -n 1); \
	if [ "$${result}" = "1.1.1.1" ] || [ "$${result}" = "1.0.0.1" ]; then \
        echo "DNS check passed: $${result}"; \
		docker stop test-dns && docker rm test-dns || true; \
    else \
        echo "DNS check failed: $${result}"; \
		docker logs test-dns; \
        exit 1; \
    fi

.PHONY: save-image
save-image:
	rm -rf *.tar
	for arch in $(ARCHITECTURES); do \
		docker container prune -f && docker image prune -af || true; \
		docker pull --platform linux/$${arch} ghcr.io/newcoderlife/coredns:latest; \
		docker save ghcr.io/newcoderlife/coredns:latest -o coredns_docker_$${arch}.tar; \
	done
	ls