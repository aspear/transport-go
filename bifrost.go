// Copyright 2019 VMware Inc.
package main

import (
    "bifrost/bridge"
    "bifrost/bus"
    "encoding/json"
    "github.com/go-stomp/stomp"
    "log"
    "os"
    "os/signal"
    "syscall"
)

var stop = make(chan bool)

var conn *stomp.Conn
var subscribed = make(chan bool)

// these are the default options that work with RabbitMQ
var options []func(*stomp.Conn) error = []func(*stomp.Conn) error{
    stomp.ConnOpt.Login("guest", "guest"),
    stomp.ConnOpt.Host("/"),
}

func main() {

    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigs
        stop <- true
    }()

    // connect to appfabric STOMP over WebSocket
    bc := bridge.NewBrokerConnector()
    config := &bridge.BrokerConnectorConfig{
        Username:   "guest",
        Password:   "guest",
        ServerAddr: "localhost:8090",
        WSPath:     "/fabric",
        UseWS:      true}

    c, _ := bc.Connect(config)

    sub, _ := c.Subscribe("/topic/simple-stream")
    count := 0
    count2 := 0

    //connect to local rabbit STOMP over TCP
    rBc := bridge.NewBrokerConnector()
    configR := &bridge.BrokerConnectorConfig{
        Username:   "guest",
        Password:   "guest",
        ServerAddr: "localhost:61613"}

    rConn, _ := rBc.Connect(configR)

    sub2, _ := rConn.Subscribe("/queue/fishsticks")

    handler := func() {
        for {
            f := <-sub.C
            r := &bridge.Response{}
            d := f.Payload.([]byte)
            json.Unmarshal(d, &r)
            count++
            log.Printf("WebSocket Message: %s", r.Payload.(string))

            cf := &bus.MessageConfig{Destination: "/queue/fishsticks", Payload: "Chicken Nugget"}
            rq := bus.GenerateRequest(cf)
            rConn.SendMessage("/queue/fishsticks", rq)

            if count >= 3 {
                sub.Unsubscribe()
                c.Disconnect()
                break
            }
        }
    }

    handler2 := func() {
        for {
            f := <-sub2.C
            count2++
            log.Printf("TCP Message: %s %v", string(f.Payload.([]byte)), count2)
            if count2 >= 3 {
                log.Println("cancelling sub ")
                sub2.Unsubscribe()
                rConn.Disconnect()
                break
            }
        }
        stop <- true
    }

    go handler()
    go handler2()

    //bf := bus.GetBus()
    //channel := "some-channel"
    //bf.GetChannelManager().CreateChannel(channel)
    //
    //// listen for a single request on 'some-channel'
    //requestHandler, _ := bf.ListenRequestStream(channel)
    //requestHandler.Handle(
    //    func(msg *bus.Message) {
    //        pingContent := msg.Payload.(string)
    //        fmt.Printf("\nPing: %s\n", pingContent)
    //
    //        // send a response back.
    //        bf.SendResponseMessage(channel, pingContent, msg.DestinationId)
    //    },
    //    func(err error) {
    //        // something went wrong...
    //    })
    //
    //// send a request to 'some-channel' and handle a single response
    //responseHandler, _ := bf.RequestOnce(channel, "Woo!")
    //responseHandler.Handle(
    //    func(msg *bus.Message) {
    //        fmt.Printf("Pong: %s\n", msg.Payload.(string))
    //    },
    //    func(err error) {
    //        // something went wrong...
    //    })

    // fire the request.

    //go recvMessages(subscribed)

    // conn, _ := stomp.Dial("tcp", *serverAddr, options...)
    // go listen(conn)
    //
    // // wait until we know the receiver has subscribed
    // <-subscribed
    // println("Subscribed!")
    // //go send(conn)
    //

    <-stop
    log.Printf("exiting")

}
