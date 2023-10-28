FROM golang:1.20
COPY . /go/src/omiewatcher
WORKDIR /go/src/omiewatcher
RUN go install -ldflags="-s -w" ./...
WORKDIR /go/src/omiewatcher/
CMD ["omiewatcher"]