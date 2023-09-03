package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
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

var mux sync.Mutex
var sent_count uint32 = 0

var wg sync.WaitGroup

var numCPU int = 0

func main() {

	numCPU = runtime.NumCPU()

	// Create the pool
	pool = MyPool.NewConnectionPool(
		&MyPool.Config{
			MaxResource:       uint64(numCPU) * 16, // Max connection of the pool
			MinIdleResource:   8,                   // Min idle connection of the pool
			Constructor:       CreateNewConnection, // Function to create new connection
			Destructor:        CloseConnection,     // Function to close connection
			ReconnectInterval: 5,                   // Time interval to reconnect if the connection of the pool are lost (in seconds)
			IdleKeepAlive:     120,                 // The time duration to remove the connection of the pool if that connection is not use (in seconds)
		})

	server_address, _ := net.ResolveTCPAddr("tcp", "tcp-cpools-service:8080")
	// configure of connection
	// connection_config_ctx is the Context Object, this variable can be used to manage life cycle
	connection_config_ctx := context.WithValue(context.Background(), addr_key, server_address)
	connection_config_ctx = context.WithValue(connection_config_ctx, keep_alive_period_key, time.Duration(30))
	// connection_config_ctx can use to close the pool (optional)
	go pool.Start(connection_config_ctx)

	time.Sleep(3 * time.Second)

	defer pool.Close()

	const n = 50000
	var tps uint32 = 0
	var delta_time float64 = 0
	var start time.Time

	// using MQTT for publish TPS of the pool

	const mqtt_broker = "tcp://mosquitto-service:1883"
	const topic = "sensor/benchmark"

	mqtt_connectLostHandler := func(client MQTT.Client, err error) {
		mylog.Printf("WARNING: Connect lost to MQTT server: %v\n", err)
	}
	mqtt_messagePubHandler := func(client MQTT.Client, msg MQTT.Message) {
		mylog.Printf("{topic: %s, message: %s}\n", msg.Topic(), string(msg.Payload()))
	}
	mqtt_connectHandler := func(client MQTT.Client) {
		mylog.Println("INFO: Connected to MQTT Broker %s\n", mqtt_broker)
	}

	opts := MQTT.NewClientOptions()
	opts.AddBroker(mqtt_broker)
	opts.SetClientID("CpoolC")
	opts.SetDefaultPublishHandler(mqtt_messagePubHandler)
	opts.OnConnect = mqtt_connectHandler
	opts.OnConnectionLost = mqtt_connectLostHandler
	opts.AutoReconnect = true

	mqtt_client := MQTT.NewClient(opts)
	if token := mqtt_client.Connect(); token.Wait() && token.Error() != nil {
		mylog.Fatalf("MQTT connect %s\n", token.Error())
	}

	for {
		sent_count = 0
		start = time.Now()
		RunTest(n)
		delta_time = time.Since(start).Seconds()
		tps = uint32(float64(sent_count) / delta_time)

		if token := mqtt_client.Publish(topic, 0, false, fmt.Sprintf(`{"TPS": %d}`, tps)); token.Wait() && token.Error() != nil {
			mylog.Printf("ERROR: MQTT Publish %s\n", token.Error())
		}
	}

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

func encapsulation_message(mess datatype.UTF8String, id datatype.Unsigned32) ([]byte, error) {
	msg := diam.NewRequest(diam.Accounting, 0, nil)
	sessionID := mess
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, sessionID)
	msg.NewAVP(avp.AcctInterimInterval, avp.Mbit, 0, id)
	return msg.Serialize()
}

func RunTest(n int) {
	num_goroutine := 16 * numCPU

	for i := 0; i < n; i += num_goroutine {

		for j := 0; j < num_goroutine; j++ {
			wg.Add(1)
			go TestSendRequest(uint32(i))
		}
		wg.Wait()

	}
}

func TestSendRequest(test_n uint32) {

	defer wg.Done()

	request, err := encapsulation_message(datatype.UTF8String("test_request_message"), datatype.Unsigned32(test_n))
	if err != nil {
		mylog.Println(err)
	}

	resource, err := pool.SendRequest(request)
	if err != nil {
		mylog.Println(err)
		return
	}
	//mylog.Println("SendRequest")

	_, err = diam.ReadMessage(resource.Value().(*net.TCPConn), dict.Default)
	if err != nil {
		mylog.Println(err)
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			resource.Release()
			return
		}
		resource.Destroy()
		return
	}

	//mylog.Printf("\n%v\n", response.String())

	mux.Lock()
	sent_count++
	mux.Unlock()

	resource.Release()

}

// func PoolInfo() {
// 	js := make(map[string]uint64)
// 	defer gw.Done()
// 	for {
// 		js["AcquiredResources"] = pool.NumActiveResource()
// 		js["IdleResources"] = pool.NumIdleResource()
// 		js["TotalResources"] = pool.NumResource()
// 		js["MaxResources"] = pool.Config().MaxResource
// 		jsonData, _ := json.Marshal(js)
// 		fmt.Println(string(jsonData))
// 		time.Sleep(time.Second)
// 	}
// }
