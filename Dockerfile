# Build
FROM golang:alpine AS build

RUN apk add --no-cache -U build-base git make

RUN mkdir -p /src

WORKDIR /src

# Copy Makefile
COPY Makefile ./

# Install deps
RUN make deps

# Copy go.mod and go.sum and install and cache dependencies
COPY go.mod .
COPY go.sum .

# Copy sources
COPY *.go ./
COPY ./client/*.go ./client/
COPY ./cmd/msgbusd/*.go ./cmd/pubsub/
COPY ./cmd/msgbus/*.go ./cmd/main.go

# Version/Commit (there there is no .git in Docker build context)
# NOTE: This is fairly low down in the Dockerfile instructions so
#       we don't break the Docker build cache just be changing
#       unrelated files that actually haven't changed but caused the
#       COMMIT value to change.
ARG VERSION="0.0.0"
ARG COMMIT="HEAD"

# Build client binary
RUN make cli VERSION=$VERSION COMMIT=$COMMIT

# Build server binary
RUN make server VERSION=$VERSION COMMIT=$COMMIT

# Runtime
FROM alpine:latest

RUN apk --no-cache -U add su-exec shadow ca-certificates tzdata

ENV PUID=1000
ENV PGID=1000

RUN addgroup -g "${PGID}" PubSubGo && \
    adduser -D -H -G PubSubGo -h /var/empty -u "${PUID}" PubSubGo && \
    mkdir -p /data && chown -R PubSubGo:PubSubGo /data


VOLUME /data

WORKDIR /
    
# force cgo resolver
ENV GODEBUG=netdns=cgo

COPY --from=build /src/PubSubGo /usr/local/bin/PubSubGo
COPY --from=build /src/PubSubGo /usr/local/bin/PubSubGo

COPY .dockerfiles/entrypoint.sh /init

ENTRYPOINT ["/init"]
CMD ["PubSubGo"]