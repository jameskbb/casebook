FROM golang:1.22 AS build
WORKDIR /src
COPY . .
RUN go build -o /out/svc ./cmd/svc

FROM gcr.io/distroless/base
COPY --from=build /out/svc /usr/local/bin/svc
COPY deploy/staging/svc.conf /etc/svc/svc.conf
COPY deploy/staging/entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
