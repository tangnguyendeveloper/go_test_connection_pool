package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
	MyPool "github.com/tangnguyendeveloper/ConnectionPool/pool"
)

var pool *MyPool.ConnectionPool

type myKeyType int

var mylog = log.New(os.Stdout, "[ClientTest] ", log.Ldate|log.Ltime)

const (
	addr_key myKeyType = iota

	// in seconds
	keep_alive_period_key myKeyType = iota
)

var gw sync.WaitGroup // for test

func main() {

	// Create the pool
	pool = MyPool.NewConnectionPool(
		&MyPool.Config{
			MaxResource:       4,                   // Max connection of the pool
			MinIdleResource:   2,                   // Min idle connection of the pool
			Constructor:       CreateNewConnection, // Function to create new connection
			Destructor:        CloseConnection,     // Function to close connection
			ReconnectInterval: 5,                   // Time interval to reconnect if the connection of the pool are lost (in seconds)
			IdleKeepAlive:     120,                 // The time duration to remove the connection of the pool if that connection is not use (in seconds)
		})

	server_address, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	// configure of connection
	// connection_config_ctx is the Context Object, this variable can be used to manage life cycle
	connection_config_ctx := context.WithValue(context.Background(), addr_key, server_address)
	connection_config_ctx = context.WithValue(connection_config_ctx, keep_alive_period_key, time.Duration(30))
	// connection_config_ctx can use to close the pool (optional)
	go pool.Start(connection_config_ctx)

	gw.Add(1)
	go PoolInfo() // for test

	time.Sleep(3 * time.Second) // for test

	defer pool.Close()

	for i := 0; i < 10; i++ {
		go TestSend()
		go TestSendMultiple()
	}

	for i := 0; i < 20; i++ {
		go TestSendRequest()
	}

	gw.Wait()
	//time.Sleep(time.Hour)

}

func CreateNewConnection(ctx context.Context) (net.Conn, error) {
	server_address := ctx.Value(addr_key).(*net.TCPAddr)
	keep_alive_period := ctx.Value(keep_alive_period_key).(time.Duration)
	connection, err := net.DialTCP("tcp", nil, server_address)
	if err == nil {
		connection.SetKeepAlive(true)
		connection.SetKeepAlivePeriod(keep_alive_period * time.Second)
	}
	return connection, err
}

func CloseConnection(value net.Conn) { value.Close() }

func encapsulation_message(mess datatype.UTF8String) ([]byte, error) {
	msg := diam.NewRequest(diam.Accounting, 0, nil)
	sessionID := mess
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, sessionID)
	return msg.Serialize()
}

func TestSend() {
	// encapsulation message
	paylpad, err := encapsulation_message(datatype.UTF8String("test_send_message"))
	if err != nil {
		mylog.Println(err)
		return
	}

	err = pool.SendSingle(paylpad)
	if err != nil {
		mylog.Println(err)
	}
	mylog.Println("SendSingle")
}

func TestSendMultiple() {
	// encapsulation message
	paylpad, err := encapsulation_message(datatype.UTF8String("test_send_Multiple_message"))
	if err != nil {
		mylog.Println(err)
		return
	}

	paylpads := [][]byte{paylpad, paylpad}

	err = pool.SendMultiple(paylpads)
	if err != nil {
		mylog.Println(err)
	}
	mylog.Println("SendMultiple")
}

func TestSendRequest() {

	request, err := encapsulation_message(datatype.UTF8String("test_request_message"))
	if err != nil {
		mylog.Println(err)
	}

	resource, err := pool.SendRequest(request)
	if err != nil {
		mylog.Println(err)
		return
	}
	mylog.Println("SendRequest")

	response, err := diam.ReadMessage(resource.Value().(*net.TCPConn), dict.Default)
	if err != nil {
		mylog.Println(err)
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			resource.Release()
			return
		}
		resource.Destroy()
		return
	}

	mylog.Printf("\n%v\n", response.String())
	time.Sleep(5 * time.Second) // for test
	resource.Release()

}

func PoolInfo() {
	js := make(map[string]uint64)
	defer gw.Done()
	for {
		js["AcquiredResources"] = pool.NumActiveResource()
		js["IdleResources"] = pool.NumIdleResource()
		js["TotalResources"] = pool.NumResource()
		js["MaxResources"] = pool.Config().MaxResource
		jsonData, _ := json.Marshal(js)
		fmt.Println(string(jsonData))
		time.Sleep(time.Second)
	}
}
