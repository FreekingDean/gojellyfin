FROM --platform=$BUILDPLATFORM golang:1.25 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

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

FROM arigaio/atlas:1.3.0 AS atlas

FROM alpine:3.22

RUN apk add --no-cache ca-certificates ffmpeg && adduser -DH gojellyfin

COPY --from=atlas /atlas /usr/local/bin/atlas
COPY --from=build /out/ /usr/local/bin/
COPY entrypoint.sh /usr/local/bin/

USER gojellyfin

EXPOSE 8081

ENTRYPOINT ["entrypoint.sh"]
CMD ["server"]
