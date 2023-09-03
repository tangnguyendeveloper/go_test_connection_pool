FROM golang:1.17

WORKDIR $GOPATH/src/CpooS

COPY . .

RUN go mod download

RUN go build -o /TCPConnectionPoolServer

EXPOSE 8080

CMD [ "/TCPConnectionPoolServer" ]