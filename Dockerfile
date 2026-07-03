FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go build -trimpath -ldflags="-s -w" -o /out/thirdcupd ./cmd/thirdcupd

FROM alpine:3.22

RUN adduser -D -H -s /sbin/nologin thirdcupd
USER thirdcupd

COPY --from=build /out/thirdcupd /usr/local/bin/thirdcupd

ENTRYPOINT ["thirdcupd"]
