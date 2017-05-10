FROM golang:1.8.0-alpine

COPY . /go/src/thrawler
WORKDIR /go/src/thrawler
RUN apk add --update git ca-certificates \
  && go get . \
  && go test . \
  && CGO_ENABLED=0 GOOS=linux go build -a -tags netgo -ldflags '-w' . \
  && mkdir build/ \
  && cp /go/src/thrawler/thrawler build/

ENTRYPOINT tar cC /go/src/thrawler/ build
