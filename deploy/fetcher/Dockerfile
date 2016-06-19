FROM scratch

ADD https://mkcert.org/generate/ /etc/ssl/certs/ca-certificates.crt

ADD fetcher /fetcher

CMD [ "/fetcher" ]

