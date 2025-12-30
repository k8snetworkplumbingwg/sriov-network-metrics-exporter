FROM docker.io/golang:1.25-alpine as builder

RUN apk add --no-cache --virtual build-dependencies build-base linux-headers git
COPY ./ /usr/src/sriov-network-metrics-exporter
WORKDIR /usr/src/sriov-network-metrics-exporter
RUN make clean && make build

FROM docker.io/alpine:3.22
COPY --from=builder /usr/src/sriov-network-metrics-exporter/build/* /usr/bin/
RUN apk update && apk add --no-cache ca-certificates && update-ca-certificates && apk add --no-cache openssl
EXPOSE 9808
ENTRYPOINT ["sriov-exporter"]