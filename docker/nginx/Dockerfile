# Nginx + ngx_brotli（动态压缩 + 静态 .br 预压缩）
FROM alpine:3.20 AS builder

RUN apk add --no-cache build-base pcre2-dev zlib-dev openssl-dev brotli-dev wget git

ENV NGINX_VERSION=1.27.5
WORKDIR /src

RUN wget -q "https://nginx.org/download/nginx-${NGINX_VERSION}.tar.gz" \
    && tar xf "nginx-${NGINX_VERSION}.tar.gz"

RUN git clone --depth 1 --recursive https://github.com/google/ngx_brotli

WORKDIR /src/nginx-${NGINX_VERSION}
RUN ./configure \
      --with-compat \
      --add-dynamic-module=../ngx_brotli \
    && make modules

FROM nginx:1.27-alpine

COPY --from=builder /src/nginx-1.27.5/objs/ngx_http_brotli_filter_module.so /etc/nginx/modules/
COPY --from=builder /src/nginx-1.27.5/objs/ngx_http_brotli_static_module.so /etc/nginx/modules/
