BUMP_VERSION := $(GOPATH)/bin/bump_version
MEGACHECK := $(GOPATH)/bin/megacheck

SHELL = /bin/bash -o pipefail

$(MEGACHECK):
	go get -u honnef.co/go/tools/cmd/megacheck

lint: | $(MEGACHECK)
	go vet ./...
	$(MEGACHECK) --ignore='github.com/kevinburke/google-oauth-handler/lib.go:S1002' ./...

test: lint
	go test ./...

race-test: lint
	go test -race ./...

install:
	go install ./...

$(BUMP_VERSION):
	go get -u github.com/Shyp/bump_version

release: race-test | $(BUMP_VERSION)
	bump_version minor lib.go
