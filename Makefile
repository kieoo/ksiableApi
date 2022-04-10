export GO111MODULE=on
BUILD_DATE=$(shell date +%Y-%m-%dT%H:%M:%S%z)
GIT_TAG=1.0

.PHONY: build
build: clean
	go build -ldflags "-X main.GitTag=$(GIT_TAG) -X main.BuildTime=$(BUILD_DATE)" ./cmd/main.go && \
  mkdir -p build/bin && mv main build/bin/ksiableApi && \
  cp deploy_ksiable.sh build/ && chmod +x build/deploy_ksiable.sh

.PHONY: clean
clean:
	rm -rf build;