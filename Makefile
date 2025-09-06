.PHONY: test test-simple cover coverage build_cli build clean

## test: runs all tests with colors
test:
	@go run scripts/test-runner.go

## test-simple: runs all tests without colors
test-simple:
	@go test -v ./...

## cover: open coverage in browser
cover:
	@go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

## coverage: displays test coverage
coverage:
	@go test -cover ./...

## build_cli: builds the command line tool gemquick and copies it to myapp
build_cli:
	@go build -o ../myapp/gq ./cmd/cli

## build: builds the command line tool dist directory
build:
	@go build -o ./dist/gq ./cmd/cli

clean:
	@rm -rf ./dist/*