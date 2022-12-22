FROM alpine:latest as cert-machine

RUN apk add --update docker openrc
RUN rc-update add docker booot

RUN docker pull miigotu/certbot-dns-godaddy

COPY env.sh /usr/bin/
RUN chmod a+x /usr/bin/env.sh
RUN /usr/bin/env.sh
RUN mkdir var
RUN mkdir etc
RUN chmod a+rw -R var/ etc/
RUN docker run --rm \
  -v  $(pwd)/var:/var/lib/letsencrypt \
  -v  $(pwd)/etc:/etc/letsencrypt \
  --cap-drop=all \
  miigotu/certbot-dns-godaddy certbot certonly -v \
    --authenticator dns-godaddy \
    --dns-godaddy-propagation-seconds 90 \
    --dns-godaddy-credentials /var/lib/letsencrypt/config.ini \
    --keep-until-expiring --non-interactive --expand \
    --server https://acme-v02.api.letsencrypt.org/directory \
    --agree-tos --email $(EMAIL) \
    -d $(DOMAIN)