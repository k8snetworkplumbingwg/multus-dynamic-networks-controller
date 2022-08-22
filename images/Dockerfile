FROM quay.io/projectquay/golang:1.18 as builder
ENV GOPATH=/go
RUN mkdir -p $GOPATH/src/github.com/maiqueb/multus-dynamic-networks-controller
WORKDIR $GOPATH/src/github.com/maiqueb/multus-dynamic-networks-controller
COPY . .
RUN GOOS=linux CGO_ENABLED=0 go build -o /dynamic-networks-controller ./cmd/dynamic-networks-controller

FROM registry.access.redhat.com/ubi8/ubi-minimal
COPY --from=builder /dynamic-networks-controller /dynamic-networks-controller
