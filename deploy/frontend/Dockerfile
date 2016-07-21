FROM alpine

RUN apk add --update \
		git \
		ca-certificates \
    && rm -rf /var/cache/apk/*

ADD frontend /frontend
ADD testrepo /testrepo

CMD [ "/frontend" ]

