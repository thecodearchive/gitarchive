export GOPATH    := $(PWD)/.GOPATH
unexport GOBIN
IMPORT_PATH      := github.com/thecodearchive/gitarchive

.PHONY: all clean
all: bin/fetcher bin/drinker bin/clone bin/migrate_cache
clean:
	rm -r .GOPATH/bin .GOPATH/pkg

.PHONY: bin/fetcher bin/drinker bin/clone bin/migrate_cache
bin/fetcher:
	@go install -v github.com/thecodearchive/gitarchive/cmd/fetcher
bin/drinker:
	@go install -v github.com/thecodearchive/gitarchive/cmd/drinker
bin/clone:
	@go install -v github.com/thecodearchive/gitarchive/cmd/clone
bin/migrate_cache:
	@CGO_ENABLED=0 go build -v -o ${@} $(PWD)/.GOPATH/src/$(IMPORT_PATH)/cmd/drinker/migrate_cache.go
