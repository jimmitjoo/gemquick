## test: runs all tests
test:
	@go test -v ./...

## cover: open coverage in browser
cover:
	@go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

## coverage: displays test coverage
coverage:
	@go test -cover ./...

## build_cli: builds the command line tool gemquick and copies it to myapp
build_cli:
	@go build -o ../myapp/gemquick ./cmd/cli

## build: builds the command line tool dist directory
build:
	@go build -o ./dist/gemquick ./cmd/cli