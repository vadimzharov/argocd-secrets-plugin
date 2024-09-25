FROM golang:1.23.1-alpine3.20 AS build_deps

ARG TARGETARCH

RUN apk add --no-cache git

WORKDIR /usr/src
ENV GO111MODULE=on

COPY go.mod .
COPY go.sum .

RUN go mod download 

FROM build_deps AS build

#ARG IMAGE_ARCH=arm

ARG ARM_VERSION=7

ENV GOARCH=$IMAGE_ARCH

ENV GOARM=$ARM_VERSION

COPY . .

RUN CGO_ENABLED=0 go build -o secrets-plugin -ldflags '-w -extldflags "-static"' ./cmd/secrets-plugin

FROM alpine:3.20.3

COPY --from=build /usr/src/secrets-plugin /usr/local/bin/secrets-plugin

ENTRYPOINT ["secrets-plugin"]