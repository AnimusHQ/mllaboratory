FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /bin/userspace ./open/demo/userspace

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=build /bin/userspace /bin/userspace
ENV USERSPACE_HTTP_ADDR=:8090
EXPOSE 8090
HEALTHCHECK --interval=5s --timeout=2s --retries=20 CMD wget -qO- http://localhost:8090/healthz >/dev/null || exit 1
ENTRYPOINT ["/bin/userspace"]
