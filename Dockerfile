FROM --platform=$BUILDPLATFORM golang:1.23 AS build

WORKDIR /go/src/app
COPY go.mod go.sum ./
RUN go mod download
ADD . /go/src/app
ARG TARGETARCH TARGETOS
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildvcs=true -o /go/bin/app .

FROM gcr.io/distroless/static
COPY --from=build /go/bin/app /
WORKDIR /
ENTRYPOINT ["/app"]
