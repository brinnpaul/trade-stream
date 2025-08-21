FROM golang:1.24-alpine

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

#RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
#    -ldflags="-w -s" \
#    -a -installsuffix cgo \
#    -o trade-stream .

RUN go build -o trade-stream .

EXPOSE 8080

CMD ["./trade-stream"]