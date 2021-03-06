# build
FROM            golang:1.16-alpine as builder
RUN             apk add --no-cache git gcc musl-dev make
ENV             GO111MODULE=on
WORKDIR         /go/src/moul.io/dl
COPY            go.* ./
RUN             go mod download
COPY            . ./
RUN             make install

# minimalist runtime
FROM            alpine:3.13.5
COPY            --from=builder /go/bin/dl /bin/
ENTRYPOINT      ["/bin/dl"]
CMD             []
