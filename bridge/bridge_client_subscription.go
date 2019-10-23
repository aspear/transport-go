// Copyright 2019 VMware Inc.

package bridge

import (
    "go-bifrost/model"
    "github.com/go-stomp/stomp"
    "github.com/go-stomp/stomp/frame"
    "github.com/google/uuid"
    "sync"
)

// BridgeClientSub is a client subscription that encapsulates message and error channels for a subscription
type BridgeClientSub struct {
    C           chan *model.Message // MESSAGE payloads
    E           chan *model.Message // ERROR payloads.
    Id          *uuid.UUID
    Destination string
    Client      *BridgeClient
    subscribed  bool
    lock        sync.RWMutex
}

// Send an UNSUBSCRIBE frame for subscription destination.
func (cs *BridgeClientSub) Unsubscribe() {
    cs.lock.Lock()
    cs.subscribed = false
    cs.lock.Unlock()
    unsubscribeFrame := frame.New(frame.UNSUBSCRIBE,
        frame.Id, cs.Id.String(),
        frame.Destination, cs.Destination,
        frame.Ack, stomp.AckAuto.String())

    cs.Client.SendFrame(unsubscribeFrame)
}
