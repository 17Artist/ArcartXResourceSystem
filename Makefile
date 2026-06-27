BINARY      := arcartx-resource
DIST        := dist
LDFLAGS     := -s -w
GOFLAGS     := -trimpath
CGO_ENABLED := 0

.PHONY: all build run clean linux linux-arm64 windows release tidy vet fmt

## 本机平台构建
build:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) .

## 本机运行（开发用）
run:
	go run .

## Linux x86_64
linux:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-amd64 .

## Linux arm64（树莓派 / ARM 云主机）
linux-arm64:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=arm64 \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-arm64 .

## Windows x86_64
windows:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=windows GOARCH=amd64 \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-windows-amd64.exe .

## 一次性产出所有发布平台
release: linux linux-arm64 windows
	@echo "构建完成，产物位于 $(DIST)/"

tidy:
	go mod tidy

vet:
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -rf $(DIST) $(BINARY) $(BINARY).exe
