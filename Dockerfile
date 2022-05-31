FROM golang:alpine as builder

ENV HTTP_PROXY $http_proxy
ENV HTTPS_PROXY $https_proxy
RUN apk add --no-cache --virtual build-dependencies build-base=0.5-r2 linux-headers=5.10.41-r0 git=2.34.2-r0
COPY ./ /usr/src/sriov-network-metrics-exporter
WORKDIR /usr/src/sriov-network-metrics-exporter
RUN make clean && make build

FROM alpine:3.16
COPY --from=builder /usr/src/sriov-network-metrics-exporter/bin/* /usr/bin/
RUN apk update && apk add --no-cache ca-certificates=20211220-r0 && update-ca-certificates && apk add --no-cache openssl=1.1.1o-r0
EXPOSE 9808
ENTRYPOINT ["sriov-exporter"]
