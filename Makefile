export GOPATH    := $(CURDIR)/.GOPATH
unexport GOBIN
IMPORT_PATH      := github.com/thecodearchive/gitarchive

.PHONY: all clean
all: bin/fetcher bin/drinker bin/backpanel bin/clone bin/frontend
clean:
	rm -rf .GOPATH/bin .GOPATH/pkg deploy/fetcher/fetcher deploy/drinker/drinker deploy/backpanel/backpanel deploy/frontend/frontend

.PHONY: bin/fetcher bin/drinker bin/clone bin/migrate_cache bin/backpanel bin/frontend
bin/fetcher:
	@go install -v github.com/thecodearchive/gitarchive/cmd/fetcher
bin/drinker:
	@go install -v github.com/thecodearchive/gitarchive/cmd/drinker
bin/backpanel:
	@go install -v github.com/thecodearchive/gitarchive/cmd/backpanel
bin/clone:
	@go install -v github.com/thecodearchive/gitarchive/cmd/clone
bin/frontend:
	@go install -v github.com/thecodearchive/gitarchive/cmd/frontend
bin/migrate_cache:
	@CGO_ENABLED=0 go build -v -o ${@} $(CURDIR)/.GOPATH/src/$(IMPORT_PATH)/cmd/drinker/migrate_cache.go

.PHONY: deploy-fetcher deploy-drinker deploy-backpanel deploy-frontend
deploy-fetcher:
	GOOS=linux GOARCH=amd64 go build -i -o deploy/fetcher/fetcher github.com/thecodearchive/gitarchive/cmd/fetcher
	docker build -t gcr.io/code-archive/fetcher:latest deploy/fetcher
	gcloud docker push gcr.io/code-archive/fetcher:latest
deploy-drinker:
	GOOS=linux GOARCH=amd64 go build -i -o deploy/drinker/drinker github.com/thecodearchive/gitarchive/cmd/drinker
	docker build -t gcr.io/code-archive/drinker:latest deploy/drinker
	gcloud docker push gcr.io/code-archive/drinker:latest
deploy-backpanel:
	GOOS=linux GOARCH=amd64 go build -i -o deploy/backpanel/backpanel github.com/thecodearchive/gitarchive/cmd/backpanel
	docker build -t gcr.io/code-archive/backpanel:latest deploy/backpanel
	gcloud docker push gcr.io/code-archive/backpanel:latest
deploy-frontend:
	GOOS=linux GOARCH=amd64 go build -i -o deploy/frontend/frontend github.com/thecodearchive/gitarchive/cmd/frontend
	docker build -t gcr.io/code-archive/frontend:latest deploy/frontend
	gcloud docker push gcr.io/code-archive/frontend:latest
