FROM golang:1.6-alpine

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

RUN buildDeps='git gcc libc-dev libgcc make sqlite-dev' \
    set -x && apk update && apk add $buildDeps \
    && cd / && git clone --depth 1 https://github.com/thecodearchive/gitarchive \
    && cd /gitarchive && make bin/drinker \
    && cp bin/drinker /usr/local/bin/ && cd / \
    && apk del $buildDeps && rm -rf /var/cache/apk/* && rm -rf /gitarchive \
    && echo "Build complete."

ADD client-config.json /root/.config/camlistore/client-config.json

CMD [ "/usr/local/bin/drinker" ]

