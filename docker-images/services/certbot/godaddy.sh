docker pull miigotu/certbot-dns-godaddy

docker run --rm \
  -v  $(pwd)/var:/var/lib/letsencrypt \
  -v  $(pwd)/etc:/etc/letsencrypt \
  --cap-drop=all \
  miigotu/certbot-dns-godaddy certbot certonly -v \
    --authenticator dns-godaddy \
    --dns-godaddy-propagation-seconds 90 \
    --dns-godaddy-credentials /var/lib/letsencrypt/config.ini \
    --keep-until-expiring --non-interactive --expand \
    --server https://acme-v02.api.letsencrypt.org/directory \
    --agree-tos --email "clay.natalie@mountaineersupport.com" \
    -d '*.test.mountaineersupport.com'