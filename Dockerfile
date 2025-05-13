FROM docker.io/golang:1.23.2-alpine as build


WORKDIR /app

COPY go.mod go.sum /app/
RUN go mod tidy

COPY . .

RUN go build -o bin cmd/server/main.go

FROM alpine:3.19

COPY --from=build /app/bin /app/bin

CMD ["/app/bin"]
