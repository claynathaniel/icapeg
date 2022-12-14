FROM alpine:latest as squid-proxy
#LABEL maintainer="alatas@gmail.com"
LABEL mantainer="clay.natalie@mountaineersupport.com"

ENV CN=proxy.mountaineersupport.local \
    O=proxy \
    OU=proxy \
    C=US

ARG all_proxy

ENV http_proxy=$all_proxy \
    https_proxy=$all_proxy

RUN apk add --no-cache \
    squid \
    openssl  \
    ca-certificates

RUN  update-ca-certificates

COPY start.sh /usr/local/bin/
COPY openssl.cnf.add /etc/ssl
COPY conf/squid*.conf /etc/squid/

RUN cat /etc/ssl/openssl.cnf.add >> /etc/ssl/openssl.cnf

RUN chmod +x /usr/local/bin/start.sh

EXPOSE 3128
EXPOSE 4128

ENTRYPOINT["/usr/local/bin/start.sh"]
