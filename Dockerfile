FROM caddy:2.10.0-builder-alpine AS builder

COPY ./caddy-meter-module /caddy-meter-module

RUN xcaddy build \
  --with github.com/mugabe/caddy-meter-module=/caddy-meter-module

FROM caddy:2.10.0-alpine

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
