.PHONY: generate package test image
generate:
	go run github.com/a-h/templ/cmd/templ@v0.3.1070 generate
	go generate ./internal/runner ./cmd/snapshot-server
	python3 scripts/schemas.py
	gofmt -w cmd internal

package: generate
	@mkdir -p dist/package/schemas dist/package/assets
	@for target in darwin-arm64 linux-arm64 linux-amd64; do \
	  os=$${target%-*}; arch=$${target#*-}; mkdir -p dist/package/bin/$$target; \
	  for binary in ui backend; do CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -o dist/package/bin/$$target/$$binary ./cmd/$$binary || exit 1; done; \
	done
	@for arch in amd64 arm64; do \
	  CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -trimpath -o dist/collector-$$arch ./cmd/collector || exit 1; \
	  CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -trimpath -o dist/snapshot-server-$$arch ./cmd/snapshot-server || exit 1; \
	done
	cp plugin.json dist/package/
	cp schemas/*.json dist/package/schemas/
	cp assets/*.css dist/package/assets/
	go run ./cmd/package-info dist/package

test: generate
	go test -race ./...
	go vet ./...

image: package
	docker build -t balemoh-image-upgrades:dev .
