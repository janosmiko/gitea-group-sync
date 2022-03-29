FROM golang:1.17.8-bullseye AS build

WORKDIR /go/src/github.com/janosmiko/gitea-ldap-sync/

COPY go.mod go.sum ./

RUN go mod download -x

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o gitea-ldap-sync .

FROM debian:bullseye-slim

COPY --from=build /go/src/github.com/janosmiko/gitea-ldap-sync/gitea-ldap-sync /usr/bin/gitea-ldap-sync

RUN apt-get update && apt-get install -y ca-certificates \
    && rm -fr /var/cache/apt/

ENTRYPOINT ["/usr/bin/gitea-ldap-sync"]
