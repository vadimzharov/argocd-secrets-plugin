FROM golang:1.23.1-alpine3.20 AS build

ARG TARGETARCH
ARG ARM_VERSION=7

RUN apk add --no-cache git

WORKDIR /usr/src
ENV GO111MODULE=on
ENV GOARCH=$TARGETARCH
ENV GOARM=$ARM_VERSION

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o secrets-plugin -ldflags '-w -extldflags "-static"' ./cmd/secrets-plugin

FROM alpine:3.20.3

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=build /usr/src/secrets-plugin /usr/local/bin/secrets-plugin
USER appuser

LABEL maintainer="Vadim Zharov <vzharov@gmail.com>" \
      description="ArgoCD Secrets Plugin" \
      version="1.0.0"

ENTRYPOINT ["secrets-plugin"]