FROM golang:latest as builder

RUN go get github.com/tools/godep

WORKDIR /go/src/go-swagger-http-worker

COPY Godeps/ Godeps/
RUN godep restore

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app .

FROM alpine:latest
RUN apk --no-cache add ca-certificates curl

WORKDIR /root/

COPY --from=builder /go/src/go-swagger-http-worker/app .

CMD ["./app"]
