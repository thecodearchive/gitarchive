export GOPATH    := $(PWD)/.GOPATH
IMPORT_PATH      := github.com/thecodearchive/gitarchive

.PHONY: all clean
all: bin/fetcher bin/drinker bin/clone
clean:
	rm -r .GOPATH/bin .GOPATH/pkg

.PHONY: bin/fetcher bin/drinker bin/clone
bin/fetcher:
	@go install -v github.com/thecodearchive/gitarchive/cmd/fetcher
bin/drinker:
	@go install -v github.com/thecodearchive/gitarchive/cmd/drinker
bin/clone:
	@go install -v github.com/thecodearchive/gitarchive/cmd/clone
