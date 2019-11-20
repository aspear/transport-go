// Copyright 2019 VMware Inc.
package main

import (
    "encoding/json"
    "fmt"
    "github.com/google/uuid"
    "go-bifrost/bridge"
    "go-bifrost/bus"
    "go-bifrost/model"
    "log"
    "math/rand"
    "os"
    "strconv"
    "time"
    "sync"
    "reflect"
    "github.com/urfave/cli"
    "go-bifrost/service"
    "go-bifrost/stompserver"
)

var addr = "localhost:8090"

func main() {
    app := cli.NewApp()
    app.Name = "Bifrost demo app"
    app.Usage = "Demonstrates different features of the Bifrost bus"
    app.Commands = []*cli.Command{

        {
            Name: "demo",
            Usage: "Run Demo - Connect to local service. You first need to start the service with 'service' command.",
            Flags: []cli.Flag{
                &cli.BoolFlag{
                    Name:  "tcp",
                    Usage: "Use TCP connection ",
                },
            },
            Action: func(c *cli.Context) error {
                runDemoApp(c)
                return nil
            },
        },
        {
            Name: "cal",
            Usage: "Call Calendar service for the time on appfabric.vmware.com",
            Action: func(c *cli.Context) error {
                runDemoCal()
                return nil
            },
        },
        {
            Name: "service",
            Usage: "Run Service - Run local service",
            Flags: []cli.Flag{
                &cli.BoolFlag{
                    Name:  "tcp",
                    Usage: "Use TCP connection ",
                },
            },
            Action: func(c *cli.Context) error {
                runLocalFabricBroker(c)
                return nil
            },
        },
        {
            Name: "store",
            Usage: "Open galactic store from appfabric.vmware.com",
            Action: func(c *cli.Context) error {
                runDemoStore()
                return nil
            },
        },
        {
            Name: "fabric-services",
            Usage: "Starts a couple of demo fabric services locally",
            Flags: []cli.Flag{
                &cli.BoolFlag{
                    Name:  "localhost",
                    Usage: "Use localhost Bifrost broker",
                },
            },
            Action: func(c *cli.Context) error {
                runDemoFabricServices(c)
                return nil
            },
        },
    }

    err := app.Run(os.Args)
    if err != nil {
        log.Fatal(err)
    }
}

func runDemoCal() {
    // get a pointer to the bus.
    b := bus.GetBus()

    // get a pointer to the channel manager
    cm := b.GetChannelManager()

    channel := "calendar-service"
    cm.CreateChannel(channel)

    // create done signal
    var done = make(chan bool)

    // listen to stream RESPONSE messages coming in.
    h, err := b.ListenStream(channel)

    if err != nil {
        log.Panicf("unable to listen to channel stream, error: %e", err)
    }

    // handle response from calendar service.
    h.Handle(
        func(msg *model.Message) {

            // unmarshal the payload into a Response object (used by fabric services)
            r := &model.Response{}
            d := msg.Payload.([]byte)
            json.Unmarshal(d, &r)
            fmt.Printf("got time response from service: %s\n", r.Payload.(string))
            done <- true
        },
        func(err error) {
            log.Panicf("error received on channel %e", err)
        })

    // create a broker connector config, in this case, we will connect to the application fabric demo endpoint.
    config := &bridge.BrokerConnectorConfig{
        Username:   "guest",
        Password:   "guest",
        UseWS: true,
        WSPath: "/fabric",
        ServerAddr: "appfabric.vmware.com" }

    // connect to broker.
    c, err := b.ConnectBroker(config)
    if err != nil {
        log.Panicf("unable to connect to fabric broker, error: %e", err)
    }
    fmt.Println("Connected to fabric broker!")

    // mark our local channel as galactic and map it to our connection and the /topic/calendar-service destination
    err = cm.MarkChannelAsGalactic(channel, "/topic/" + channel, c)
    if err != nil {
        log.Panicf("unable to map local channel to broker destination: %e", err)
    }

    // create request
    id := uuid.New();
    r := &model.Request{}
    r.Request = "time"
    r.Id = &id
    m, _ := json.Marshal(r)
    fmt.Println("Requesting time from calendar service")

    // send request.
    c.SendMessage("/pub/" + channel, m)

    // wait for done signal
    <-done
    fmt.Printf("\nDone.\n\n")

    // mark channel as local (unsubscribe from all mappings)
    err = cm.MarkChannelAsLocal(channel)
    if err != nil {
        log.Panicf("unable to unsubscribe, error: %e", err)
    }

    err = c.Disconnect()
    if err != nil {
        log.Panicf("unable to disconnect, error: %e", err)
    }
}

type SampleMessageItem struct {
    From string `json:"from"`
    Message string `json:"message"`
}

func (mi SampleMessageItem) print() {
    fmt.Println("FROM:", mi.From)
    fmt.Println("Message:", mi.Message)
}

func runDemoStore() {
    // get a pointer to the bus.
    b := bus.GetBus()

    // create a broker connector config, in this case, we will connect to the application fabric demo endpoint.
    config := &bridge.BrokerConnectorConfig{
        Username:   "guest",
        Password:   "guest",
        UseWS: true,
        WSPath: "/fabric",
        ServerAddr: "appfabric.vmware.com" }

    // connect to broker.
    c, err := b.ConnectBroker(config)
    if err != nil {
        log.Panicf("unable to connect to fabric broker, error: %e", err)
    }
    fmt.Println("Connected to fabric broker:", config.ServerAddr)

    err = b.GetStoreManager().ConfigureStoreSyncChannel(c, "/topic/", "/pub/")
    if err != nil {
        log.Panicf("unable to configure store sync channel, error: %e", err)
    }

    var motdStore bus.BusStore
    motdStore, err = b.GetStoreManager().OpenGalacticStoreWithItemType(
            "messageOfTheDayStore", c, reflect.TypeOf(SampleMessageItem{}))
    if err != nil {
        log.Panicf("failed to open galactic store, error: %e", err)
    }

    wg := sync.WaitGroup{}
    wg.Add(1)
    motdStore.WhenReady(func() {
        wg.Done()
    })

    wg.Wait()

    originalItem := motdStore.GetValue("messageOfTheDay").(SampleMessageItem)
    originalItem.print()

    storeStream := motdStore.OnChange("messageOfTheDay")
    storeStream.Subscribe(func(change *bus.StoreChange) {
        if change.IsDeleteChange {
            fmt.Println("Item removed: ", change.Id)
        } else {
            fmt.Println("Store item changed: ")
            change.Value.(SampleMessageItem).print()
        }
        wg.Done()
    })

    wg.Add(1)
    motdStore.Put("messageOfTheDay",
        SampleMessageItem{
            Message: "updated sample message",
            From: "test user",
        }, "update")
    wg.Wait()

    wg.Add(1)
    motdStore.Put("messageOfTheDay", originalItem, "update")
    wg.Wait()

    // Local stores
    localStringStore := b.GetStoreManager().CreateStore("localStringStore")
    localMessageStore := b.GetStoreManager().CreateStore("localSampleMessageStore")

    // use async transaction to wait for the two local stores.
    tr := b.CreateAsyncTransaction()
    tr.WaitForStoreReady(localStringStore.GetName())
    tr.WaitForStoreReady(localMessageStore.GetName())

    wg.Add(1)
    tr.OnComplete(func(responses []*model.Message) {
        fmt.Println("Local stores initialized")
        fmt.Println("localStringStore:")
        fmt.Println(responses[0].Payload)
        fmt.Println("localSampleMessageStore:")
        fmt.Println(responses[1].Payload)
        wg.Done()
    })

    fmt.Println("Waiting for local stores...")
    tr.Commit()

    localStringStore.Populate(map[string]interface{} {
        "key1": "value1",
        "key2": "value2",
        "key3": "value3",
    })

    // copy the values from the galactic motdStore to the local
    // store
    localMessageStore.Populate(motdStore.AllValuesAsMap())

    b.GetStoreManager().DestroyStore("messageOfTheDayStore")

    wg.Wait()
}

func runDemoApp(ctx *cli.Context) {

    // ensure unique message ping message ids
    rand.Seed(time.Now().UTC().UnixNano())

    // get a pointer to the bus.
    b := bus.GetBus()

    // get a pointer to the channel manager
    cm := b.GetChannelManager()

    channel := "my-stream"
    cm.CreateChannel(channel)

    // create done signal
    var done = make(chan bool)

    // listen to stream of messages coming in on channel.
    h, err := b.ListenStream(channel)
    if err != nil {
        log.Panicf("unable to listen to channel stream, error: %e", err)
    }

    count := 0

    // listen for five messages and then exit, send a completed signal on channel.
    h.Handle(
        func(msg *model.Message) {

            // unmarshal the payload into a Response object (used by fabric services)
            r := &model.Response{}
            d := msg.Payload.([]byte)
            json.Unmarshal(d, &r)
            fmt.Printf("Stream ticked from local broker: %s\n", r.Payload.(string))
            count++
            if count >=5 {
                done <- true
            }
        },
        func(err error) {
            log.Panicf("error received on channel %e", err)
        })

    // create a broker connector config, in this case, we will connect to local fabric broker
    var config *bridge.BrokerConnectorConfig
    if ctx.Bool("tcp") {
        config = &bridge.BrokerConnectorConfig{
            Username:   "guest",
            Password:   "guest",
            ServerAddr: addr }
    } else {
        config = &bridge.BrokerConnectorConfig{
            Username:   "guest",
            Password:   "guest",
            UseWS: true,
            WSPath: "/fabric",
            ServerAddr: addr }
    }

    // connect to broker.
    c, err := b.ConnectBroker(config)
    if err != nil {
        log.Panicf("unable to connect to local broker, error: %e", err)
    }
    fmt.Println("Connected to local broker!")

    // mark our local channel as galactic and map it to our connection and the /topic/ping-service
    // running locally
    err = cm.MarkChannelAsGalactic(channel, "/topic/ping-service", c)
    if err != nil {
        log.Panicf("unable to map local channel to broker destination: %e", err)
    }

    fmt.Printf("\nSending 5 public messages to broker, every 500ms\n\n")
    time.Sleep(1 * time.Second)
    for i := 0; i < 5; i++ {
        pl := "ping--" + strconv.Itoa(rand.Intn(10000000))
        r := &model.Response{Payload: pl}
        m, _ := json.Marshal(r)
        c.SendMessage("/pub/ping-service", m)
        time.Sleep(500 * time.Millisecond)
    }

    // wait for done signal
    <-done

    fmt.Printf("\nDone.\n\n")

    // mark channel as local (unsubscribe from all mappings)
    err = cm.MarkChannelAsLocal(channel)
    if err != nil {
        log.Panicf("unable to unsubscribe, error: %e", err)
    }

    privateChannel := "my-private-channel"
    cm.CreateChannel(privateChannel)
    // mark the privateChannel channel as galactic and map it to /user/queue/ping-service
    err = cm.MarkChannelAsGalactic(privateChannel, "/user/queue/ping-service", c)
    if err != nil {
        log.Panicf("unable to map local channel to broker destination: %e", err)
    }

    // listen to stream of messages coming in on channel.
    ph, err := b.ListenStream(privateChannel)
    if err != nil {
        log.Panicf("unable to listen to channel stream, error: %e", err)
    }

    count = 0
    // listen for five messages and then exit, send a completed signal on channel.
    ph.Handle(
        func(msg *model.Message) {
            // unmarshal the payload into a Response object (used by fabric services)
            r := &model.Response{}
            d := msg.Payload.([]byte)
            json.Unmarshal(d, &r)
            fmt.Printf("Stream ticked from local broker on private channel: %s\n", r.Payload.(string))
            count++
            if count >=5 {
                done <- true
            }
        },
        func(err error) {
            log.Panicf("error received on channel %e", err)
        })

    fmt.Printf("\nSending 5 private messages to broker, every 500ms\n\n")
    time.Sleep(1 * time.Second)
    for i := 0; i < 5; i++ {

        pl := "ping--" + strconv.Itoa(rand.Intn(10000000))
        r := &model.Response{Payload: pl}
        m, _ := json.Marshal(r)
        c.SendMessage("/pub/queue/ping-service", m)
        time.Sleep(500 * time.Millisecond)
    }

    // wait for done signal
    <-done

    // mark channel as local (unsubscribe from all mappings)
    err = cm.MarkChannelAsLocal(privateChannel)
    if err != nil {
        log.Panicf("unable to unsubscribe, error: %e", err)
    }

    err = c.Disconnect()
    if err != nil {
        log.Panicf("unable to disconnect, error: %e", err)
    }
}

func runDemoFabricServices(c *cli.Context) {
    b := bus.GetBus()

    fmt.Println("Registering calendar and simple-stream services.")
    simpleTickerService := &simpleStreamTickerService{ channelName: "simple-stream" }
    service.GetServiceRegistry().RegisterService(simpleTickerService, simpleTickerService.channelName)
    service.GetServiceRegistry().RegisterService(&calendarService{}, "calendar-service")

    if c.Bool("localhost") {
        service.GetServiceRegistry().SetGlobalRestServiceBaseHost("localhost:8090")
    }

    wg := sync.WaitGroup{}
    mh,_ := b.ListenStream("calendar-service")
    mh.Handle(func(message *model.Message) {
        resp := message.Payload.(model.Response)
        if resp.Error {
            fmt.Println("Received error response from calendar-service:", resp.ErrorMessage)
        } else {
            fmt.Println("Received response from calendar-service:", resp.Payload)
        }
        wg.Done()
    }, func(e error) {})

    wg.Add(1)
    if c.Bool("localhost") {
        fmt.Println("Sending REST request to the http://localhost:8090/rest/samples/calendar/time")
    } else {
        fmt.Println("Sending REST request to the http://appfabric.vmware.com/rest/samples/calendar/time")
    }
    b.SendRequestMessage("calendar-service", model.Request{Request: "rest-time"}, nil)
    wg.Wait()

    wg.Add(1)
    fmt.Println("Sending \"time\" request to the calendar service")
    b.SendRequestMessage("calendar-service", model.Request{Request: "time"}, nil)
    wg.Wait()
    wg.Add(1)
    fmt.Println("Sending \"date\" request to the calendar service")
    b.SendRequestMessage("calendar-service", model.Request{Request: "date"}, nil)
    wg.Wait()
    wg.Add(1)
    fmt.Println("Sending invalid request to the calendar service")
    b.SendRequestMessage("calendar-service", model.Request{Request: "invalid-request"}, nil)
    wg.Wait()

    counter := 0
    wg.Add(10)
    fmt.Println("Subscribing to the simple-stream channel and waiting for 10 messages...")
    tickerMh, _ := b.ListenStream(simpleTickerService.channelName)
    tickerMh.Handle(func(message *model.Message) {
        resp := message.Payload.(model.Response)
        if resp.Error {
            fmt.Println("Received error response from simple-stream:", resp.ErrorMessage)
        } else {
            fmt.Println("Received response from simple-stream:", resp.Payload)
        }
        wg.Done()

        counter++
        if counter == 5 {
            b.SendRequestMessage(simpleTickerService.channelName, model.Request{Request: "offline"}, nil)
            fmt.Println("Temporary stopping the simple-stream service for 3 seconds...")
            time.Sleep(3 * time.Second)
            fmt.Println("Resuming the simple-stream...")
            b.SendRequestMessage(simpleTickerService.channelName, model.Request{Request: "online"}, nil)
        }

    }, func(e error) {})

    wg.Wait()
}

type simpleStreamTickerService struct {
    online bool
    channelName string
}

func (fs *simpleStreamTickerService) HandleServiceRequest(request *model.Request, core service.FabricServiceCore) {
    switch request.Request {
    case "online":
        fs.online = true
    case "offline":
        fs.online = false
    default:
        core.HandleUnknownRequest(request)
    }
}

func (fs *simpleStreamTickerService) Init(core service.FabricServiceCore) error {
    fs.online = true
    ticker := time.NewTicker(500 * time.Millisecond)
    go func() {
        for {
            <-ticker.C
            if fs.online {
                now := time.Now()
                id := uuid.New()
                response := model.Response{
                    Payload: fmt.Sprintf("ping-%d", now.Nanosecond() + now.Second()),
                    Id: &id,
                }
                core.Bus().SendResponseMessage(fs.channelName, response, nil)
            }
        }
    }()
    return nil
}

type calendarService struct {}

func (cs *calendarService) HandleServiceRequest(
        request *model.Request, core service.FabricServiceCore) {

    switch request.Request {
    case "time":
        core.SendResponse(request, time.Now().Format("03:04:05.000 AM (-0700)"))
    case "date":
        core.SendResponse(request, time.Now().Format("Mon, 02 Jan 2006"))
    case "rest-time":
        core.RestServiceRequest(&service.RestServiceRequest{
            Url: "http://appfabric.vmware.com/rest/samples/calendar/time",
            HttpMethod: "POST",
            Body: model.Request{
                Request: "time",
            },
            ResponseType: reflect.TypeOf(model.Response{}),
        }, func(response *model.Response) {
            core.SendResponse(request, response.Payload.(model.Response).Payload)
        }, func(response *model.Response) {
            core.SendErrorResponse(request, response.ErrorCode, response.ErrorMessage)
        })
    default:
        core.HandleUnknownRequest(request)
    }
}

type pingService struct {}

func (cs *pingService) HandleServiceRequest(
        request *model.Request, core service.FabricServiceCore) {

    requestMessage := request.Payload.(string)
    core.SendResponse(request, requestMessage + "-pong")
}

func runLocalFabricBroker(c *cli.Context) {
    fmt.Println("Service Starting...")

    service.GetServiceRegistry().RegisterService(&pingService{}, "ping-service")

    var err error
    var connectionListener stompserver.RawConnectionListener
    if c.Bool("tcp") {
        connectionListener, err = stompserver.NewTcpConnectionListener(addr)
    } else {
        connectionListener, err = stompserver.NewWebSocketConnectionListener(addr, "/fabric", nil)
    }

    if err == nil {
        err = bus.GetBus().StartFabricEndpoint(connectionListener, bus.EndpointConfig{
            TopicPrefix:      "/topic",
            AppRequestPrefix: "/pub",
            UserQueuePrefix: "/user/queue",
            AppRequestQueuePrefix: "/pub/queue",
            Heartbeat:        60000, // 6 seconds
        })
    }

    if err != nil {
        fmt.Println("Failed to start local fabric broker", err)
    }
}
