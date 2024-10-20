VERSION := $(shell git rev-parse --short HEAD)
ARCHITECTURES := amd64 arm64
TAG ?= $(shell git describe --tags --abbrev=0)
SAVE_ARCH ?= linux/arm64
USER := $(shell whoami)
SUDO := $(if $(filter root, $(USER)),,sudo)

all: prepare-docker test

.PHONY: force-push
force-push:
	@$(info TAG=$(TAG))
	@git add . && git commit --amend --no-edit && git push -f && git tag $(TAG) -f && git push origin -f $(TAG);

.PHONY: go-version
go-version:
	@cat ./tmp/coredns/.go-version

.PHONY: install-dependency
install-dependency:
	@$(SUDO) apt-get install debhelper base-files dpkg-dev jq fakeroot dnsutils -y

.PHONY: clone-dependency
clone-dependency:
	@rm -rf tmp && mkdir tmp
	@git clone --depth 1 https://github.com/coredns/coredns.git tmp/coredns

.PHONY: build-coredns
build-coredns: install-dependency clone-dependency
	@$(info VERSION=$(VERSION))
	@echo "pforward:github.com/newcoderlife/pforward" >> ./tmp/coredns/plugin.cfg
	@cat ./tmp/coredns/plugin.cfg
	@$(MAKE) -C ./tmp/coredns/ gen
	@cd ./tmp/coredns && go get "github.com/newcoderlife/pforward@$(VERSION)"
	@$(MAKE) -C ./tmp/coredns/ -f Makefile.release build LINUX_ARCH="amd64 arm64"

.PHONY: pack-coredns
pack-coredns: build-coredns
	@$(info TAG=$(TAG))
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
	sleep 60
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

.PHONY: build-local
build-local:
	@rm -rf coredns && rm -rf build && mkdir build
	@git clone https://github.com/coredns/coredns.git build/coredns --depth 1
	@echo "pforward:github.com/newcoderlife/pforward" >> ./build/coredns/plugin.cfg
	@$(MAKE) -C ./build/coredns/ gen
	@cd ./build/coredns/ && go mod edit -replace=github.com/newcoderlife/pforward=../../ && go mod tidy
	@$(MAKE) -C ./build/coredns/ coredns && cp ./build/coredns/coredns .

.PHONY: Corefile.test
Corefile.test:
	@rm -rf Corefile.test
	@echo ".:53 {" > Corefile.test
	@echo "    metadata" >> Corefile.test
	@echo "" >> Corefile.test
	@echo "    pforward ./rules/noncn tls://1.1.1.1 {" >> Corefile.test
	@echo "        tls_servername cloudflare-dns.com" >> Corefile.test
	@echo "    }" >> Corefile.test
	@echo "" >> Corefile.test
	@echo "    pforward . tls://223.5.5.5 {" >> Corefile.test
	@echo "        tls_servername dns.alidns.com" >> Corefile.test
	@echo "    }" >> Corefile.test
	@echo "" >> Corefile.test
	@echo "    log . \"{type} {name} {/pforward/upstream} {/pforward/backup} {duration} {/pforward/response/ip}\"" >> Corefile.test
	@echo "    errors {" >> Corefile.test
	@echo "        stacktrace" >> Corefile.test
	@echo "    }" >> Corefile.test
	@echo "}" >> Corefile.test

.PHONY: run-local
run-local: build-local Corefile.test
	@rm -rf rules && git clone https://github.com/newcoderlife/ruleset.git rules --depth 1 && touch rules/local.noncn rules/local.cn
	@./coredns -conf ./Corefile.test
