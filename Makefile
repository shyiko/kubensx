SHELL := /bin/bash -o pipefail
VERSION := $(shell git describe --tags --abbrev=0)
fetch:
	go get \
	github.com/mitchellh/gox \
	github.com/golang/dep/cmd/dep \
	github.com/modocache/gover \
	github.com/aktau/github-release && \
	dep ensure

clean:
	rm -f ./kubensx
	rm -rf ./build

fmt:
	gofmt -l -s -w `find . -type f -name '*.go' -not -path "./vendor/*"`

test:
	go vet `go list ./... | grep -v /vendor/`
	SRC=`find . -type f -name '*.go' -not -path "./vendor/*" -not -path "./.tmp/*"` && \
		gofmt -l -s $$SRC | read && gofmt -l -s -d $$SRC && exit 1 || true
	go test -v `go list ./... | grep -v /vendor/` | grep -v "=== RUN"

test-coverage:
	go list ./... | grep -v /vendor/ | xargs -L1 -I{} sh -c 'go test -coverprofile `basename {}`.coverprofile {}' && \
	gover && \
	go tool cover -html=gover.coverprofile -o coverage.html && \
	rm *.coverprofile

test-spec:
	spec/run.sh > /tmp/kubensx-spec-log 2>&1 && diff /tmp/kubensx-spec-log spec/expected.log | cat -A

build:
	go build -ldflags "-X main.version=${VERSION}"

build-release:
	gox -verbose \
	-ldflags "-X main.version=${VERSION}" \
	-osarch="windows/amd64 linux/amd64 darwin/amd64" \
	-output="release/{{.Dir}}-${VERSION}-{{.OS}}-{{.Arch}}" .

sign-release:
	for file in $$(ls release/kubensx-${VERSION}-*); do gpg --detach-sig --sign -a $$file; done

publish: clean build-release sign-release
	test -n "$(GITHUB_TOKEN)" # $$GITHUB_TOKEN must be set
	github-release release --user shyiko --repo kubensx --tag ${VERSION} \
	--name "${VERSION}" --description "${VERSION}" && \
	github-release upload --user shyiko --repo kubensx --tag ${VERSION} \
	--name "kubensx-${VERSION}-windows-amd64.exe" --file release/kubensx-${VERSION}-windows-amd64.exe; \
	github-release upload --user shyiko --repo kubensx --tag ${VERSION} \
	--name "kubensx-${VERSION}-windows-amd64.exe.asc" --file release/kubensx-${VERSION}-windows-amd64.exe.asc; \
	for qualifier in darwin-amd64 linux-amd64 ; do \
		github-release upload --user shyiko --repo kubensx --tag ${VERSION} \
		--name "kubensx-${VERSION}-$$qualifier" --file release/kubensx-${VERSION}-$$qualifier; \
		github-release upload --user shyiko --repo kubensx --tag ${VERSION} \
		--name "kubensx-${VERSION}-$$qualifier.asc" --file release/kubensx-${VERSION}-$$qualifier.asc; \
	done
