// Copyright 2019 VMware Inc.
package util

import "bifrost/model"

const (
    ChannelCreatedEvt          int = 0
    ChannelDestroyedEvt        int = 1
    ChannelSubscriberJoinedEvt int = 2
    ChannelSubscriberLeftEvt   int = 3
    ChannelMessageEvt          int = 4
    ChannelErrorEvt            int = 5
    ChannelIsGalacticEvt       int = 6
    ChannelIsLocalEvt          int = 7
    BrokerConnectedEvtWs       int = 8
    BrokerConnectedEvtTcp      int = 9
    BrokerDisconnectedWs       int = 10
    BrokerDisconnectedTcp      int = 11
)

type MonitorEvent struct {
    EventType int
    Message   *model.Message
    Channel   string
}

// Create a new monitor event
func NewMonitorEvent(evtType int, channel string, message *model.Message, ) *MonitorEvent {
    return &MonitorEvent{EventType: evtType, Message: message, Channel: channel}
}
