FROM alpine:latest as squid-proxy
#LABEL maintainer="alatas@gmail.com"
LABEL mantainer="clay.natalie@mountaineersupport.com"

ENV CN=proxy.mountaineersupport.com \
    O=proxy \
    OU=proxy \
    C=US

ARG all_proxy

ENV http_proxy=$all_proxy \
    https_proxy=$all_proxy

RUN apk add --no-cache \
    squid \
    openssl  \
    ca-certificates \
    certbot \
    python3 \
    python3-dev \
    py3-pip \
    build-base \
    libressl-dev \
    musl-dev \
    libffi-dev \
    rust \
    cargo

RUN pip3 install pip --upgrade

RUN mkdir /etc/letsencrypt

RUN  update-ca-certificates

RUN certbot --manual --preferred-challenges dns certonly

#COPY start.sh /usr/local/bin/
COPY keys_setup.py /usr/local/bin
COPY openssl.cnf.add /etc/ssl
COPY conf/squid*.conf /etc/squid/

RUN cat /etc/ssl/openssl.cnf.add >> /etc/ssl/openssl.cnf


EXPOSE 3128
EXPOSE 4128

#ENTRYPOINT ["/usr/local/bin/start.sh"]
ENTRYPOINT ["/bin/sh"]