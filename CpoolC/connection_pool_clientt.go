package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
	"github.com/jackc/puddle/v2"
)

type myKeyType int

const addr_key myKeyType = 1

func main() {

	const (
		minPoolSize                  uint  = 2
		maxPoolSize                  int32 = 8 // 16, 32, ...
		reconnect_interval_in_second       = 5
	)

	server_address, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")

	// Create a TCP connection pool
	// Max TCP connection of pool is maxPoolSize

	pool, err := puddle.NewPool(
		&puddle.Config[net.Conn]{
			Constructor: CreateNewConnection,
			Destructor:  CloseConnection,
			MaxSize:     maxPoolSize,
		},
	)

	if err != nil {
		log.Fatal(err)
	}

	// Optional:   Init minPoolSize TCP connection for the pool
	err = InitConnection(pool, minPoolSize, server_address)
	if err != nil {
		pool.Close()
		log.Fatal(err)
	}

	go ReconnectForever(pool, maxPoolSize, server_address, reconnect_interval_in_second)

	// simulating send 100 packages via pool

	var wg sync.WaitGroup
	wg.Add(101)

	for i := 1; i < 101; i++ {
		go RunTask(pool, fmt.Sprintf("task_%d", i), server_address, &wg)
		time.Sleep(50 * time.Millisecond)
	}
	// Wait for all task is finish
	wg.Wait()

	// optional
	PrintPoolState(pool)

	pool.Close()
}

// #####################################################

// #####################################################

func CreateNewConnection(ctx context.Context) (net.Conn, error) {
	server_address := ctx.Value(addr_key).(*net.TCPAddr)
	connection, err := net.DialTCP("tcp", nil, server_address)
	if err == nil {
		connection.SetKeepAlive(true)
		connection.SetKeepAlivePeriod(30 * time.Second)
	}
	return connection, err
}

func CloseConnection(value net.Conn) { value.Close() }

func InitConnection(pool *puddle.Pool[net.Conn], num_connection uint, server_address *net.TCPAddr) error {

	for i := 0; i < int(num_connection); i++ {
		ctx := context.Background()
		ctx = context.WithValue(ctx, addr_key, server_address)

		err := pool.CreateResource(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func ReconnectForever(pool *puddle.Pool[net.Conn], maxPoolSize int32, server_address *net.TCPAddr, reconnect_interval_in_second time.Duration) {

	one := make([]byte, 1)

	timeout := 3 * time.Second

	for {

		// If the pool have no one connection, it should be to reconnect.

		if pool.Stat().TotalResources() == 0 {
			InitConnection(pool, 1, server_address)
		}
		// remove connections lost

		for _, connection := range pool.AcquireAllIdle() {
			connection.Value().(*net.TCPConn).SetReadDeadline(time.Now().Add(timeout))
			_, err := connection.Value().Read(one)

			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				connection.Release()
				continue
			} else if err != nil {
				connection.Destroy()
				continue
			}

			connection.Release()

		}

		// Optional: the TCP connection will remove if it is idle.
		if pool.Stat().IdleResources() > maxPoolSize/4 {
			pool.Reset()
		}

		// in real-world .etc

		// in test
		time.Sleep(2*reconnect_interval_in_second ^ time.Second)
		PrintPoolState(pool)
	}
}

func RunTask(pool *puddle.Pool[net.Conn], message string, server_address *net.TCPAddr, wg *sync.WaitGroup) {
	defer wg.Done()

	ctx := context.Background()
	ctx = context.WithValue(ctx, addr_key, server_address)

	// Return a connection managed by pool
	// If the pool has not idle connection and the total connection of the pool are less than maxPoolSize then Acquire method will create new connection .
	res, err := pool.Acquire(ctx)
	if err != nil {
		log.Printf("ERROR Run task %s ", message)
		log.Println(err)
		return
	}

	res.Value().SetDeadline(time.Time{})

	// encapsulation message
	msg := diam.NewRequest(diam.Accounting, 0, nil)
	sessionID := datatype.UTF8String(message)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, sessionID)

	// Send message to server
	_, err = msg.WriteTo(res.Value())

	if err != nil {
		log.Println(err)
		res.Destroy()
		return
	}

	// receive message from server
	response, _ := diam.ReadMessage(res.Value(), dict.Default)

	fmt.Println("\n____________________________________________________________")
	fmt.Println(response.String())
	fmt.Println("______________________________________________________________")
	fmt.Println()

	// optional
	PrintPoolState(pool)

	time.Sleep(time.Second)

	// return the connection to the pool
	res.Release()

	// res.ReleaseUnused()  // remove the connection
}

func PrintPoolState(pool *puddle.Pool[net.Conn]) {
	js := make(map[string]int64)
	js["AcquiredResources"] = int64(pool.Stat().AcquiredResources())
	js["IdleResources"] = int64(pool.Stat().IdleResources())
	js["TotalResources"] = int64(pool.Stat().TotalResources())
	js["MaxResources"] = int64(pool.Stat().MaxResources())
	jsonData, _ := json.Marshal(js)
	fmt.Println(string(jsonData))
}
