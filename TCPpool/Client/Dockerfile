FROM golang:1.17

WORKDIR $GOPATH/src/CpooC

COPY . .

RUN go mod download

RUN go build -o /TCPConnectionPoolclient

CMD [ "/TCPConnectionPoolclient" ]