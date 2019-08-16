// Copyright 2019 VMware Inc.
package util

import (
    "bifrost/model"
    "sync"
)

var once sync.Once
var monitorInstance *MonitorStream

// Get a reference to the monitor.
func GetMonitor() *MonitorStream {
    once.Do(func() {
        m := new(MonitorStream)
        m.Stream = make(chan *MonitorEvent, 5) // give this a small buffer.
        monitorInstance = m
    })
    return monitorInstance
}

// Monitor stream exposes a channel to listen for bus events.
type MonitorStream struct {
    Stream chan *MonitorEvent
}

// Send a new monitor event without any payload to the monitor stream
func (m *MonitorStream) SendMonitorEvent(evtType int, channel string) {
    // make this non blocking, there may be no-one listening.
    select {
    case m.Stream <- NewMonitorEvent(evtType, channel, nil):
    default:
        // channel full, no-one listening, drop.
    }
}

// Send a new monitor event with a message payload to the monitor stream. This is non-blocking
// so it does not matter if there is no-one listening to the stream.
func (m *MonitorStream) SendMonitorEventData(evtType int, channel string, msg *model.Message) {
    // make this non blocking, there may be no-one listening.
    select {
    case m.Stream <- NewMonitorEvent(evtType, channel, msg):
    default:
        // channel full, no-one listening, drop.
    }
}
