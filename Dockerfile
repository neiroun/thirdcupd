FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go build -trimpath -ldflags="-s -w" -o /out/thirdcupd ./cmd/thirdcupd

FROM alpine:3.22

RUN adduser -D -H -s /sbin/nologin thirdcupd

COPY --from=build /out/thirdcupd /usr/local/bin/thirdcupd
COPY configs/thirdcupd.docker.json /etc/thirdcupd/thirdcupd.json

USER thirdcupd

EXPOSE 8374
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O - http://127.0.0.1:8374/healthz >/dev/null || exit 1

ENTRYPOINT ["thirdcupd"]
CMD ["-config", "/etc/thirdcupd/thirdcupd.json"]
