FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/cajero-web ./cmd/web

FROM alpine:3.20
RUN adduser -D app && mkdir /data && chown app /data
USER app
COPY --from=build /out/cajero-web /usr/local/bin/cajero-web
ENV ADDR=:8080 DATA_FILE=/data/banco.json
EXPOSE 8080
VOLUME /data
CMD ["cajero-web"]
