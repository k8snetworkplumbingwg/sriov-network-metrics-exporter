FROM golang:alpine as builder

ENV HTTP_PROXY $http_proxy
ENV HTTPS_PROXY $https_proxy
RUN apk add --update --virtual build-dependencies build-base linux-headers
ADD ./ /usr/src/sriov-network-metrics-exporter
RUN cd /usr/src/sriov-network-metrics-exporter && make clean && make build

FROM alpine
COPY --from=builder /usr/src/sriov-network-metrics-exporter/bin/* /usr/bin/
RUN apk update && apk add ca-certificates && update-ca-certificates && apk add openssl
EXPOSE 9808
ENTRYPOINT ["sriov-exporter"]
