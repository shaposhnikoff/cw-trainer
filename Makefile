.PHONY: build test release

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
