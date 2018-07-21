.PHONY: assets static templates

SHELL = /bin/bash -o pipefail

BUMP_VERSION := $(GOPATH)/bin/bump_version
DIFFER := $(GOPATH)/bin/differ
GENERATE_TLS_CERT := $(GOPATH)/bin/generate-tls-cert
GO_BINDATA := $(GOPATH)/bin/go-bindata
JUSTRUN := $(GOPATH)/bin/justrun
MEGACHECK := $(GOPATH)/bin/megacheck
RELEASE := $(GOPATH)/bin/github-release

# Add files that change frequently to this list.
WATCH_TARGETS = $(shell find ./static ./templates -type f)
GO_FILES = $(shell find . -name '*.go')
GO_NOASSET_FILES := $(filter-out ./assets/bindata.go,$(GO_FILES))


$(GOPATH)/bin:
	mkdir -p $(GOPATH)/bin

UNAME := $(shell uname)

$(MEGACHECK): | $(GOPATH)/bin
ifeq ($(UNAME), Darwin)
	curl --silent --location --output $(MEGACHECK) https://github.com/kevinburke/go-tools/releases/download/2018-05-12/megacheck-darwin-amd64
endif
ifeq ($(UNAME), Linux)
	curl --silent --location --output $(MEGACHECK) https://github.com/kevinburke/go-tools/releases/download/2018-05-12/megacheck-linux-amd64
endif
	chmod 755 $(MEGACHECK)

lint: $(MEGACHECK)
	go list ./... | grep -v vendor | xargs go vet
	go list ./... | grep -v vendor | xargs $(MEGACHECK)

test: lint
	go list ./... | grep -v vendor | xargs go test

check: $(GOPATH)/bin/multi-emailer
ifndef config
	$(eval config = config.yml)
endif
	multi-emailer --check --config=$(config)

race-test: lint
	go list ./... | grep -v vendor | xargs go test -race

$(GOPATH)/bin/multi-emailer: $(GO_FILES) | $(GOPATH)/bin
	go install .

serve: $(GOPATH)/bin/multi-emailer
ifndef config
	$(eval config = config.yml)
endif
	$(GOPATH)/bin/multi-emailer --config=$(config)

serve-local: | $(GOPATH)/bin/multi-emailer
	$(MAKE) serve config=local.yml

watch-local: | $(GOPATH)/bin/multi-emailer leaf.key leaf.pem
	$(MAKE) watch config=local.yml

leaf.key:
leaf.pem: | $(GENERATE_TLS_CERT)
	$(GENERATE_TLS_CERT) --host=localhost:8048

$(GENERATE_TLS_CERT): | $(GOPATH)/bin
	go get -u github.com/kevinburke/generate-tls-cert/...

$(GO_BINDATA): | $(GOPATH)/bin
	go get -u github.com/kevinburke/go-bindata/...

assets: static/license.txt static/privacy.html | $(GO_BINDATA)
	$(GO_BINDATA) -o=assets/bindata.go --nometadata --pkg=assets templates/... static/...

$(JUSTRUN): | $(GOPATH)/bin
	go get -u github.com/jmhodges/justrun

watch: | $(JUSTRUN)
	$(JUSTRUN) -v --delay=100ms -c 'make assets serve' $(WATCH_TARGETS) $(GO_NOASSET_FILES)

generate_cert:
	go run "$$(go env GOROOT)/src/crypto/tls/generate_cert.go" --host=localhost:8048,127.0.0.1:8048 --ecdsa-curve=P256 --ca=true

static/privacy.html: privacy-policy.md
	markdown privacy-policy.md > static/privacy.html

static/license.txt: LICENSE
	cp -f LICENSE static/license.txt

$(DIFFER): $(GOPATH)/bin
	curl --silent --location --output $(GOPATH)/bin/differ https://github.com/kevinburke/differ/releases/download/0.5/differ-linux-amd64 && chmod 755 $(GOPATH)/bin/differ

diff: $(DIFFER)
	$(DIFFER) $(MAKE) assets static/privacy.html

$(BUMP_VERSION):
	go get -u github.com/kevinburke/bump_version

$(RELEASE):
	go get -u github.com/aktau/github-release

# make release version=foo
release: diff test | $(BUMP_VERSION) $(RELEASE)
ifndef version
	@echo "Please provide a version"
	exit 1
endif
ifndef GITHUB_TOKEN
	@echo "Please set GITHUB_TOKEN in the environment"
	exit 1
endif
	$(BUMP_VERSION) --version=$(version) main.go
	git push origin --tags
	mkdir -p releases/$(version)
	GOOS=linux GOARCH=amd64 go build -o releases/$(version)/multi-emailer-linux-amd64 .
	GOOS=darwin GOARCH=amd64 go build -o releases/$(version)/multi-emailer-darwin-amd64 .
	GOOS=windows GOARCH=amd64 go build -o releases/$(version)/multi-emailer-windows-amd64 .
	# these commands are not idempotent so ignore failures if an upload repeats
	$(RELEASE) release --user kevinburke --repo multi-emailer --tag $(version) || true
	$(RELEASE) upload --user kevinburke --repo multi-emailer --tag $(version) --name multi-emailer-linux-amd64 --file releases/$(version)/multi-emailer-linux-amd64 || true
	$(RELEASE) upload --user kevinburke --repo multi-emailer --tag $(version) --name multi-emailer-darwin-amd64 --file releases/$(version)/multi-emailer-darwin-amd64 || true
	$(RELEASE) upload --user kevinburke --repo multi-emailer --tag $(version) --name multi-emailer-windows-amd64 --file releases/$(version)/multi-emailer-windows-amd64 || true
