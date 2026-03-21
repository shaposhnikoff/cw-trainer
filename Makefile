.PHONY: build test release
CURRENT_VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")

build:
	go build -o cw-trainer ./cmd/cw-trainer

test:
	go test ./...

# Create and push a release tag: v1.DDMMYYYY.HHMM  (valid semver)
release:
	$(eval TAG := v1.$(shell date +%d%m%Y).$(shell date +%H%M))
	@echo "Tagging $(TAG)"
	git tag $(TAG)
	git push origin $(TAG)

release-patch:
	@V=$(CURRENT_VERSION) && \
	PATCH=$$(echo $$V | awk -F. '{print $$3+1}') && \
	NEW=$$(echo $$V | awk -F. "{printf \"%s.%s.$$PATCH\", \$$1, \$$2}") && \
	git tag -a $$NEW -m "Release $$NEW" && \
	git push origin $$NEW

release-minor:
	@V=$(CURRENT_VERSION) && \
	MINOR=$$(echo $$V | awk -F. '{print $$2+1}') && \
	NEW=$$(echo $$V | awk -F. "{printf \"%s.$$MINOR.0\", \$$1}") && \
	git tag -a $$NEW -m "Release $$NEW" && \
	git push origin $$NEW

release-major:
	@V=$(CURRENT_VERSION) && \
	MAJOR=$$(echo $$V | sed 's/v//' | awk -F. '{print $$1+1}') && \
	NEW="v$$MAJOR.0.0" && \
	git tag -a $$NEW -m "Release $$NEW" && \
	git push origin $$NEW
