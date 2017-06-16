FROM golang:1.8.3-alpine3.6

RUN apk --no-cache add ca-certificates git

RUN go get github.com/tools/godep

WORKDIR /go/src/app

COPY Godeps/ Godeps/
RUN godep restore

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo .

CMD ["./app"]
