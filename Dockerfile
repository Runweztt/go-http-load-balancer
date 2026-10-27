
FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd

ARG SERVICE

RUN CGO_ENABLED=0 go build -o /server ./cmd/${SERVICE}

FROM alpine:3.20

COPY --from=build /server /server

ENTRYPOINT ["/server"]
