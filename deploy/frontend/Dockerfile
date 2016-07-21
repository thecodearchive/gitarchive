FROM scratch

ADD https://mkcert.org/generate/ /etc/ssl/certs/ca-certificates.crt

ADD frontend /frontend

CMD [ "/frontend" ]

