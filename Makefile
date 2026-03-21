.PHONY: build test release

build:
	go build -o cw-trainer ./cmd/cw-trainer

test:
	go test ./...

# Create and push a release tag: v1.0.DDMMYYYY.HHMM
release:
	$(eval TAG := v1.0.$(shell date +%d%m%Y.%H%M))
	@echo "Tagging $(TAG)"
	git tag $(TAG)
	git push origin $(TAG)
