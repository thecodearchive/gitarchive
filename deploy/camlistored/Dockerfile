FROM alpine

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go

RUN	apk update && apk add ca-certificates \
	&& rm -rf /var/cache/apk/*

RUN buildDeps='go git gcc libc-dev libgcc' \
	set -x && apk update && apk add $buildDeps \
	&& mkdir -p /go/src/ && cd /go/src/ \
	&& git clone --depth 1 https://github.com/thecodearchive/camlistore camlistore.org \
	&& cd /go/src/camlistore.org && go run make.go \
	&& cp -r ./bin/* /usr/local/bin/ \
	&& apk del $buildDeps && rm -rf /var/cache/apk/* && rm -rf /go \
	&& echo "Build complete."

ADD server-config.json /server-config.json
EXPOSE 3179
CMD [ "camlistored", "-configfile", "/server-config.json" ]
