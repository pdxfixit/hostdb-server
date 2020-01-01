FROM scratch

LABEL maintainer="Ben Sandberg <info@pdxfixit.com>" \
      name="hostdb-server" \
      vendor="PDXfixIT, LLC"

ENV GIN_MODE=release

COPY cert.pem /etc/ssl/certs/
COPY assets/ /assets/
COPY views/ /views/
COPY hostdb-server /usr/bin/
COPY config.yaml /etc/hostdb/
COPY mariadb/*.sql /mariadb/

EXPOSE 8080

ENTRYPOINT [ "/usr/bin/hostdb-server" ]
