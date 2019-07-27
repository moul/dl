GO ?= go
VERSION ?= `git describe --tags --abbrev=0 | sed s/^//`

.PHONY: install
install:
	CGO_ENABLED=0 $(GO) install -a -ldflags '-extldflags "-static"' .

.PHONY: test
test:
	echo "" > /tmp/coverage.txt
	set -e; for dir in `find . -type f -name "go.mod"  | sed -r 's@/[^/]+$$@@' | sort | uniq`; do ( set -xe; \
	  cd $$dir; \
	  $(GO) test -v -cover -coverprofile=/tmp/profile.out -covermode=atomic -race ./...; \
	  if [ -f /tmp/profile.out ]; then \
	    cat /tmp/profile.out >> /tmp/coverage.txt; \
	    rm -f /tmp/profile.out; \
	  fi); done
	mv /tmp/coverage.txt .

.PHONY: lint
lint:
	golangci-lint run --verbose ./...

.PHONY: release
release:
	goreleaser --snapshot --skip-publish --rm-dist
	@echo -n "Do you want to release? [y/N] " && read ans && [ $${ans:-N} = y ]
	goreleaser --rm-dist

.PHONY: orb-validate
orb-validate:
	circleci orb validate .circleci/config.yml

.PHONY: orb-publish
orb-publish: orb-validate
	set -x; circleci orb publish .circleci/config.yml moul/dl@${VERSION}
