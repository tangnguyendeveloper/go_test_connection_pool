package main

import (
	"fmt"
	"net"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
)

func main() {
	bind_address, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	server, err := net.ListenTCP("tcp", bind_address)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("Serve at %s\n", server.Addr())

	connection_count := 1

	for {
		client, err := server.AcceptTCP()
		if err != nil {
			fmt.Println(err)
			continue
		}
		go handleConnection(client, uint(connection_count))
		connection_count++
	}

}

func handleConnection(connection net.Conn, connectionID uint) {
	fmt.Printf("\nCreated NEW connection %s -> %s [ ID = %d ]\n\n", connection.LocalAddr(), connection.RemoteAddr(), connectionID)

	for {
		request, err := diam.ReadMessage(connection, dict.Default)
		if err != nil {
			fmt.Println(err)
			connection.Close()
			fmt.Printf("Closed connection %s -> %s [ ID = %d ]\n", connection.LocalAddr(), connection.RemoteAddr(), connectionID)
			return
		}
		fmt.Println("\n____________________________________________________________")
		fmt.Printf(" message: %s, from connectionID: %d\n", request.String(), connectionID)
		fmt.Println("______________________________________________________________")

		msg := diam.NewRequest(diam.Accounting, 0, nil)
		msg.Header.CommandFlags = 0
		sessionID := datatype.UTF8String(fmt.Sprintf("0k 200, id=%d", connectionID))
		msg.NewAVP(avp.SessionID, avp.Mbit, 0, sessionID)

		// Send response to client
		msg.WriteTo(connection)

		time.Sleep(100 * time.Microsecond)
	}
}
