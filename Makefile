.PHONY: assets static templates

SHELL = /bin/bash

DIFFER := $(shell command -v differ)
GO_BINDATA := $(shell command -v go-bindata)
JUSTRUN := $(shell command -v justrun)
STATICCHECK := $(shell command -v staticcheck)

# Add files that change frequently to this list.
WATCH_TARGETS = static/style.css templates/index.html main.go form.go

lint:
ifndef STATICCHECK
	go get -u honnef.co/go/tools/cmd/staticcheck
endif
	go vet ./...
	staticcheck ./...

test: lint
	go test ./...

race-test: lint
	go test -race ./...

serve:
ifndef config
	$(eval config = config.yml)
endif
	go install . && multi-emailer --config=$(config)

assets:
ifndef GO_BINDATA
	go get -u github.com/jteeuwen/go-bindata/...
endif
	go-bindata -o=assets/bindata.go --nometadata --pkg=assets templates/... static/...

watch:
ifndef JUSTRUN
	go get -u github.com/jmhodges/justrun
endif
	justrun -v --delay=100ms -c 'make assets serve' $(WATCH_TARGETS)

generate_cert:
	go run "$$(go env GOROOT)/src/crypto/tls/generate_cert.go" --host=localhost:8048,127.0.0.1:8048 --ecdsa-curve=P256 --ca=true

diff:
ifndef DIFFER
	go get -u github.com/kevinburke/differ
endif
	differ $(MAKE) assets

# make release version=foo
release: diff test
ifndef version
	@echo "Please provide a version"
	exit 1
endif
ifndef GITHUB_TOKEN
	@echo "Please set GITHUB_TOKEN in the environment"
	exit 1
endif
ifndef BUMP_VERSION
	go get github.com/Shyp/bump_version
endif
	bump_version --version=$(version) main.go
	git push origin --tags
	mkdir -p releases/$(version)
	GOOS=linux GOARCH=amd64 go build -o releases/$(version)/multi-emailer-linux-amd64 .
	GOOS=darwin GOARCH=amd64 go build -o releases/$(version)/multi-emailer-darwin-amd64 .
	GOOS=windows GOARCH=amd64 go build -o releases/$(version)/multi-emailer-windows-amd64 .
ifndef RELEASE
	go get -u github.com/aktau/github-release
endif
	# these commands are not idempotent so ignore failures if an upload repeats
	github-release release --user kevinburke --repo multi-emailer --tag $(version) || true
	github-release upload --user kevinburke --repo multi-emailer --tag $(version) --name multi-emailer-linux-amd64 --file releases/$(version)/multi-emailer-linux-amd64 || true
	github-release upload --user kevinburke --repo multi-emailer --tag $(version) --name multi-emailer-darwin-amd64 --file releases/$(version)/multi-emailer-darwin-amd64 || true
	github-release upload --user kevinburke --repo multi-emailer --tag $(version) --name multi-emailer-windows-amd64 --file releases/$(version)/multi-emailer-windows-amd64 || true
