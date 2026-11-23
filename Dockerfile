FROM --platform=$BUILDPLATFORM node:26-alpine AS admin

WORKDIR /src/admin

COPY admin/package.json admin/package-lock.json ./
RUN npm ci

COPY admin ./
COPY spec/jellyfin-openapi-stable.patched.json /src/spec/
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=admin /src/internal/server/admin/dist internal/server/admin/dist

# Generated code is committed, so the image never runs `go generate`: it would
# need the codegen tools and the network.

# .dockerignore drops .git, so the build cannot derive any of this itself.
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

ARG TARGETARCH
ENV CGO_ENABLED=0 GOOS=linux
ENV STAMP=github.com/FreekingDean/gojellyfin/internal/system
RUN GOARCH=$TARGETARCH go build \
      -ldflags "-X $STAMP.buildVersion=$VERSION -X $STAMP.buildCommit=$COMMIT -X $STAMP.buildDate=$DATE" \
      -o /out/ ./cmd/gojellyfin

FROM arigaio/atlas:1.4.1 AS atlas

FROM alpine:3.24

RUN apk add --no-cache ca-certificates ffmpeg && adduser -DH gojellyfin

COPY --from=atlas /atlas /usr/local/bin/atlas
COPY --from=build /out/ /usr/local/bin/
COPY entrypoint.sh /usr/local/bin/

USER gojellyfin

EXPOSE 8081

ENTRYPOINT ["entrypoint.sh"]
CMD ["server"]
