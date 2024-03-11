FROM golang:1.21-alpine as builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

COPY ./app/*.go ./

RUN GOOS=linux GOARCH=arm64 go build -mod=readonly -o server

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/server .

RUN apk --no-cache add ca-certificates tzdata

RUN adduser -D user && chown -R user /app
USER user
CMD ["/app/server"]
