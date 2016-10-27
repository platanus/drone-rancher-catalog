FROM alpine:3.4

RUN apk update && \
  apk add \
    ca-certificates \
    git \
    openssh-client && \
  rm -rf /var/cache/apk/*

ADD drone-rancher-catalog /bin/
ENTRYPOINT ["/bin/drone-rancher-catalog"]
