FROM golang:1.21 AS build
ENV GOPROXY=https://proxy.golang.org
WORKDIR /go/utils/mutating-demo
COPY . .
RUN CGO_ENABLED=0 go build -o /go/bin/mutating-demo .

FROM centos:latest
MAINTAINER "1689991551@qq.com"
COPY --from=build /go/bin/* /utils/

WORKDIR /utils
ENTRYPOINT ["/bin/sh","-c","sleep 360000000"]