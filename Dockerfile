FROM golang:1.18.0-alpine3.15 as builder

ARG VERSION

WORKDIR /go/src/lfgw/
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ENV CGO_ENABLED=0

RUN go install \
    -installsuffix "static" \
    -ldflags "                                          \
      -X main.Version=${VERSION}                        \
      -X main.GoVersion=$(go version | cut -d " " -f 3) \
      -X main.Compiler=$(go env CC)                     \
      -X main.Platform=$(go env GOOS)/$(go env GOARCH) \
    " \
    ./...

FROM alpine:3.15.3 as runtime

RUN set -x \
  && apk add --update --no-cache ca-certificates tzdata \
  && echo 'Etc/UTC' > /etc/timezone \
  && update-ca-certificates

ENV TZ=/etc/localtime                  \
    LANG=en_US.utf8                    \
    LC_ALL=en_US.UTF-8

COPY --from=builder /go/bin/lfgw /
RUN chmod +x /lfgw

RUN adduser -S appuser -u 1000 -G root
USER 1000

ENTRYPOINT ["/lfgw"]
