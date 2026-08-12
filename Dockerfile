FROM --platform=$BUILDPLATFORM golang:1.25 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Generated code is committed, so the image never runs `go generate`: it needs
# the codegen tools and the network.
ARG TARGETARCH
ENV CGO_ENABLED=0 GOOS=linux
RUN GOARCH=$TARGETARCH go build -o /out/ ./cmd/server ./cmd/tasks/migrate

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
