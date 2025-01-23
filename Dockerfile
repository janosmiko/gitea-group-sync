FROM golang:1.23.4-bullseye AS build

WORKDIR /go/src/github.com/janosmiko/gitea-ldap-sync/

COPY go.mod go.sum ./

RUN go mod download -x

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o gitea-ldap-sync .

FROM debian:bookworm-slim

COPY --from=build /go/src/github.com/janosmiko/gitea-ldap-sync/gitea-ldap-sync /usr/bin/gitea-ldap-sync

RUN <<EOF
apt-get update
apt-get install -y --no-install-recommends ca-certificates
rm -rf /var/lib/apt/lists/*
EOF

ENTRYPOINT ["/usr/bin/gitea-ldap-sync"]
