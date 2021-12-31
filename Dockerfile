FROM golang:1.16-buster AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
COPY *.go ./

ENV CGO_ENABLED 0

RUN go mod download


RUN go build -o /go-roku

##
## Deploy
##
FROM alpine

WORKDIR /

RUN mkdir assets
COPY --from=build /go-roku /go-roku
COPY assets/ assets/

EXPOSE 8000

ENTRYPOINT ["/go-roku"]
