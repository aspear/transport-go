package bridge

import (
    "bufio"
    "bytes"
    "fmt"
    "github.com/go-stomp/stomp/frame"
    "github.com/go-stomp/stomp/server"
    "github.com/gorilla/websocket"
    "github.com/stretchr/testify/assert"
    "log"
    "net"
    "net/http"
    "net/http/httptest"
    "net/url"
    "testing"
)

var upgrader = websocket.Upgrader{}

func websocketHandler(w http.ResponseWriter, r *http.Request) {
    c, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer c.Close()
    for {
        mt, message, err := c.ReadMessage()
        if err != nil {
            break
        }

        br := bytes.NewReader(message)
        sr := frame.NewReader(br)
        f, _ := sr.Read()

        var sendFrame *frame.Frame

        switch f.Command {
        case frame.CONNECT:
            sendFrame = frame.New(frame.CONNECTED,
                frame.ContentType, "application/json")

        case frame.SUBSCRIBE:
            sendFrame = frame.New(frame.MESSAGE,
                frame.Destination, f.Header.Get(frame.Destination),
                frame.ContentType, "application/json")
            sendFrame.Body = []byte("happy baby melody!")
        }
        var bb bytes.Buffer
        bw := bufio.NewWriter(&bb)
        sw := frame.NewWriter(bw)
        sw.Write(sendFrame)

        err = c.WriteMessage(mt, bb.Bytes())
        if err != nil {
            break
        }
    }
}

//var srv Server
var testBrokerAddress = ":8992"
var httpServer *httptest.Server
var tcpServer net.Listener
var webSocketURLChan = make(chan string)
var websocketURL string

func runStompBroker() {
    l, err := net.Listen("tcp", testBrokerAddress)
    if err != nil {
        log.Fatalf("failed to listen: %s", err.Error())
    }
    defer func() { l.Close() }()

    log.Println("TCP listening on", l.Addr().Network(), l.Addr().String())
    server.Serve(l)
    tcpServer = l
}

func runWebSocketEndPoint() {
    s := httptest.NewServer(http.HandlerFunc(websocketHandler))
    log.Println("WebSocket listening on", s.Listener.Addr().Network(), s.Listener.Addr().String())
    httpServer = s
    webSocketURLChan <- s.URL
    websocketURL = s.URL
}

func init() {
    go runStompBroker()
    go runWebSocketEndPoint()
}

func TestBrokerConnector_BadConfig(t *testing.T) {
    tt := []struct {
        test   string
        config *BrokerConnectorConfig
        err    error
    }{
        {
            "Missing address from config",
            &BrokerConnectorConfig{Username: "guest", Password: "guest"},
            fmt.Errorf("config invalid, config missing server address")},
        {
            "Missing username from config",
            &BrokerConnectorConfig{ServerAddr: "somewhere:000"},
            fmt.Errorf("config invalid, config missing username")},
        {
            "Missing password from config",
            &BrokerConnectorConfig{Username: "hi", ServerAddr: "somewhere:000"},
            fmt.Errorf("config invalid, config missing password")},
    }

    for _, tc := range tt {
        t.Run(tc.test, func(t *testing.T) {
            bc := NewBrokerConnector()
            c, err := bc.Connect(tc.config)
            assert.Nil(t, c)
            assert.NotNil(t, err)
            assert.Equal(t, tc.err, err)
        })
    }
}

func TestBrokerConnector_ConnectBroker(t *testing.T) {
    u := <-webSocketURLChan
    url, _ := url.Parse(u)
    host, port, _ := net.SplitHostPort(url.Host)
    testHost := host + ":" + port

    tt := []struct {
        test   string
        config *BrokerConnectorConfig
    }{
        {
            "Connect via websocket",
            &BrokerConnectorConfig{
                Username: "guest", Password: "guest", UseWS: true, WSPath: "/", ServerAddr: testHost}},
        {
            "Connect via TCP",
            &BrokerConnectorConfig{
                Username: "guest", Password: "guest", ServerAddr: testBrokerAddress}},
    }

    for _, tc := range tt {
        t.Run(tc.test, func(t *testing.T) {

            // connect
            bc := NewBrokerConnector()
            c, err := bc.Connect(tc.config)
            assert.NotNil(t, c)
            assert.Nil(t, err)
            if tc.config.UseWS {
                assert.NotNil(t, c.wsConn)
            }
            if !tc.config.UseWS {
                assert.NotNil(t, c.conn)
            }

            // disconnect
            err = c.Disconnect()
            assert.Nil(t, err)
            if tc.config.UseWS {
                assert.Nil(t, c.wsConn)
            }
            if !tc.config.UseWS {
                assert.Nil(t, c.conn)
            }

        })
    }
}

func TestBrokerConnector_ConnectBrokerFail(t *testing.T) {
    tt := []struct {
        test   string
        config *BrokerConnectorConfig
    }{
        {
            "Connect via websocket fails with bad address",
            &BrokerConnectorConfig{
                Username: "guest", Password: "guest", UseWS: true, WSPath: "/", ServerAddr: "nowhere"}},
        {
            "Connect via TCP fails with bad address",
            &BrokerConnectorConfig{
                Username: "guest", Password: "guest", ServerAddr: "somewhere"}},
    }

    for _, tc := range tt {
        t.Run(tc.test, func(t *testing.T) {
            bc := NewBrokerConnector()
            c, err := bc.Connect(tc.config)
            assert.Nil(t, c)
            assert.NotNil(t, err)
        })
    }
}

func TestBrokerConnector_Subscribe(t *testing.T) {
    url, _ := url.Parse(websocketURL)
    host, port, _ := net.SplitHostPort(url.Host)
    testHost := host + ":" + port

    tt := []struct {
        test   string
        config *BrokerConnectorConfig
    }{
        {
            "Subscribe via websocket",
            &BrokerConnectorConfig{
                Username: "guest", Password: "guest", UseWS: true, WSPath: "/", ServerAddr: testHost}},
        {
            "Subscribe via TCP",
            &BrokerConnectorConfig{
                Username: "guest", Password: "guest", ServerAddr: testBrokerAddress}},
    }

    for _, tc := range tt {
        t.Run(tc.test, func(t *testing.T) {

            // connect
            bc := NewBrokerConnector()
            c, _ := bc.Connect(tc.config)
            s, _ := c.Subscribe("/topic/test")
            if !tc.config.UseWS {
                var ping = func() {
                    c.SendMessage("/topic/test", []byte(`happy baby melody!`))
                }
                go ping()
            }
            msg := <-s.C
            ba := msg.Payload.([]byte)
            assert.Equal(t, "happy baby melody!", string(ba))

            // check re-subscribe returns same sub
            s2, _ := c.Subscribe("/topic/test")
            assert.Equal(t, s.Id.ID(), s2.Id.ID())

            c.Disconnect()
        })
    }
}

func TestBrokerConnector_SubscribeError(t *testing.T) {
    bc := NewBrokerConnector()
    c, _ := bc.Connect(nil)
    s, err := c.Subscribe("/topic/test")
    assert.NotNil(t, err)
    assert.Nil(t, s)

    fakeConn := new(Connection)
    fakeConn.useWs = true

    // check websocket connection check
    s, err = fakeConn.Subscribe("/topic/test")
    assert.NotNil(t, err)
    assert.Nil(t, s)

    // test tcp connection check
    fakeConn = new(Connection)
    s, err = fakeConn.Subscribe("/topic/test")
    assert.NotNil(t, err)
    assert.Nil(t, s)
}

func TestBrokerConnector_DisconnectNoConnect(t *testing.T) {
    bc := NewBrokerConnector()
    // connect without any config
    c, _ := bc.Connect(nil)
    err := c.Disconnect()
    assert.NotNil(t, err)
    assert.Nil(t, c)
}

func TestBrokerConnector_SendMessageOnWs(t *testing.T) {
    // u := <- webSocketURLChan
    url, _ := url.Parse(websocketURL)
    host, port, _ := net.SplitHostPort(url.Host)
    testHost := host + ":" + port

    cf := &BrokerConnectorConfig{
        Username: "guest", Password: "guest", UseWS: true, WSPath: "/", ServerAddr: testHost}

    bc := NewBrokerConnector()
    c, _ := bc.Connect(cf)
    assert.NotNil(t, c)

    e := c.SendMessage("nowhere", []byte("outhere"))
    assert.Nil(t, e)


    // try and send a message on a closed connection
    cf = &BrokerConnectorConfig{
        Username: "guest", Password: "guest", ServerAddr: testBrokerAddress}

    bc = NewBrokerConnector()
    c, _ = bc.Connect(cf)
    assert.NotNil(t, c)

    c.Disconnect()

    e = c.SendMessage("nowhere", []byte("outhere"))
    assert.NotNil(t, e)


}
