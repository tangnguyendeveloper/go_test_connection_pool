package main

import (
	"log"
	"net"
	"os"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
)

var mylog = log.New(os.Stdout, "[ServerTest] ", log.Ldate|log.Ltime)

func main() {
	bind_address, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	server, err := net.ListenTCP("tcp", bind_address)
	if err != nil {
		mylog.Println(err)
		return
	}

	mylog.Printf("Serve at %s\n", server.Addr())

	connection_count := 1

	for {
		client, err := server.AcceptTCP()
		if err != nil {
			mylog.Println(err)
			continue
		}
		go handleConnection(client, uint(connection_count))
		connection_count++
	}

}

func handleConnection(connection net.Conn, connectionID uint) {
	mylog.Printf("Created NEW connection %s -> %s [ ID = %d ]\n", connection.LocalAddr(), connection.RemoteAddr(), connectionID)

	for {
		request, err := diam.ReadMessage(connection, dict.Default)
		if err != nil {
			mylog.Println(err)
			connection.Close()
			mylog.Printf("Closed connection %s -> %s [ ID = %d ]\n", connection.LocalAddr(), connection.RemoteAddr(), connectionID)
			return
		}

		if avp, err := request.FindAVP(avp.SessionID, 0); err == nil {

			switch avp.Data.(datatype.UTF8String) {
			case datatype.UTF8String("test_send_message"), datatype.UTF8String("test_send_Multiple_message"):
				mylog.Printf(" message: %s, from connectionID: %d\n", avp.Data.String(), connectionID)
				continue
			default:
				response := request.Answer(200)
				response.WriteTo(connection)
				mylog.Printf("Responded %s to connectionID: %d\n", avp.Data.String(), connectionID)
			}
		}

		time.Sleep(100 * time.Microsecond)
	}
}
