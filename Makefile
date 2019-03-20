BUILD_DIR="./build"
COVER_PROFILE:="${BUILD_DIR}/coverage.out"

default: test build clean

build:
	go build pkg/client/client.go

test: code-check-test unit-test

code-check-test:
	go vet  ./...

unit-test:
	mkdir -p ${BUILD_DIR}
	go test -v -cover -coverprofile=${COVER_PROFILE} ./...

cover: unit-test
	go tool cover -html=${COVER_PROFILE}

clean:
	rm -rf ${BUILD_DIR}
