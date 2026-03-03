FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/ssh-deploy ./cmd/ssh-deploy

FROM alpine:3.21

# docker-cli and docker-cli-compose are required for Compose backend.
# When using Komodo backend only, these can be removed.
RUN apk add --no-cache ca-certificates docker-cli docker-cli-compose

WORKDIR /app
COPY --from=build /out/ssh-deploy /usr/local/bin/ssh-deploy

EXPOSE 2222

ENTRYPOINT ["/usr/local/bin/ssh-deploy"]
