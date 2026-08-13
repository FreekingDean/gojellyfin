FROM --platform=$BUILDPLATFORM golang:1.25 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Generated code is committed, so the image never runs `go generate`: it would
# need the codegen tools and the network.
ARG TARGETARCH
ENV CGO_ENABLED=0 GOOS=linux
RUN GOARCH=$TARGETARCH go build -o /out/ ./cmd/gojellyfin

# jellyfin-web ships no prebuilt release tarball and its npm package is a
# metadata-only stub, so the Debian package is the only official build of the
# client. It is arch-independent (_all) but upstream files it under amd64.
FROM --platform=$BUILDPLATFORM alpine:3.22 AS web

ARG JELLYFIN_WEB_VERSION=10.10.7
RUN apk add --no-cache curl dpkg && \
	curl -fsSL -o /tmp/jellyfin-web.deb \
	"https://repo.jellyfin.org/files/server/debian/stable/v${JELLYFIN_WEB_VERSION}/amd64/jellyfin-web_${JELLYFIN_WEB_VERSION}%2Bdeb12_all.deb" && \
	dpkg-deb -x /tmp/jellyfin-web.deb /out

FROM arigaio/atlas:1.3.0 AS atlas

FROM alpine:3.22

RUN apk add --no-cache ca-certificates ffmpeg && adduser -DH gojellyfin

COPY --from=atlas /atlas /usr/local/bin/atlas
COPY --from=build /out/ /usr/local/bin/
COPY --from=web /out/usr/share/jellyfin/web /usr/share/jellyfin/web
COPY entrypoint.sh /usr/local/bin/

USER gojellyfin

EXPOSE 8081

ENTRYPOINT ["entrypoint.sh"]
CMD ["server"]
