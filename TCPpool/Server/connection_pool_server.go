package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/dict"
)

var mylog = log.New(os.Stdout, "[ServerTest] ", log.Ldate|log.Ltime)

var mux sync.Mutex
var connection_count uint = 0

var mqtt_client MQTT.Client

const topic = "sensor/benchmark"

func main() {
	bind_address, _ := net.ResolveTCPAddr("tcp", ":8080")
	server, err := net.ListenTCP("tcp", bind_address)
	if err != nil {
		mylog.Println(err)
		return
	}

	mylog.Printf("Serve at %s\n", server.Addr())

	// using MQTT for publish Number connection

	const mqtt_broker = "tcp://mosquitto-service:1883"

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
	opts.SetClientID("CpoolS")
	opts.SetDefaultPublishHandler(mqtt_messagePubHandler)
	opts.OnConnect = mqtt_connectHandler
	opts.OnConnectionLost = mqtt_connectLostHandler
	opts.AutoReconnect = true

	mqtt_client = MQTT.NewClient(opts)
	if token := mqtt_client.Connect(); token.Wait() && token.Error() != nil {
		mylog.Fatalf("MQTT connect %s\n", token.Error())
	}

	go func() {

		for {
			client, err := server.AcceptTCP()
			if err != nil {
				mylog.Println(err)
				continue
			}
			go handleConnection(client)
			mux.Lock()
			connection_count++
			publish()
			mux.Unlock()
		}

	}()

	for {
		time.Sleep(2 * time.Minute)
		publish()
	}

}

func publish() {
	if token := mqtt_client.Publish(topic, 0, false, fmt.Sprintf(`{"NumConnection": %d}`, connection_count)); token.Wait() && token.Error() != nil {
		mylog.Printf("ERROR: MQTT Publish %s\n", token.Error())
	}
}

func handleConnection(connection net.Conn) {

	for {
		request, err := diam.ReadMessage(connection, dict.Default)
		if err != nil {
			connection.Close()
			mux.Lock()
			connection_count--
			publish()
			mux.Unlock()
			return
		}

		if _, err := request.FindAVP(avp.SessionID, 0); err == nil {

			_, err = request.FindAVP(avp.AcctInterimInterval, 0)
			if err != nil {
				mylog.Println(err)
				continue
			}
			response := request.Answer(200)
			response.WriteTo(connection)

		}

		time.Sleep(10 * time.Microsecond)
	}
}
