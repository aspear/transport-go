// Copyright 2019 VMware Inc.

package bus

import (
    "fmt"
    "github.com/google/uuid"
    "sync"
)

// EventBus provides access to ChannelManager, simple message sending and simple API calls for handling
// messaging and error handling over channels on the bus.
type EventBus interface {
    GetId() *uuid.UUID
    GetChannelManager() ChannelManager
    SendRequestMessage(channelName string, payload interface{}, destinationId *uuid.UUID) error
    SendResponseMessage(channelName string, payload interface{}, destinationId *uuid.UUID) error
    SendErrorMessage(channelName string, err error, destinationId *uuid.UUID) error
    ListenStream(channelName string) (MessageHandler, error)
    ListenStreamForDestination(channelName string, destinationId *uuid.UUID) (MessageHandler, error)
    ListenFirehose(channelName string) (MessageHandler, error)
    ListenRequestStream(channelName string) (MessageHandler, error)
    ListenRequestStreamForDestination(channelName string, destinationId *uuid.UUID) (MessageHandler, error)
    ListenRequestOnce(channelName string) (MessageHandler, error)
    ListenRequestOnceForDestination (channelName string, destinationId *uuid.UUID) (MessageHandler, error)
    ListenOnce(channelName string) (MessageHandler, error)
    ListenOnceForDestination(channelName string, destId *uuid.UUID) (MessageHandler, error)
    RequestOnce(channelName string, payload interface{}) (MessageHandler, error)
    RequestOnceForDestination(channelName string, payload interface{}, destId *uuid.UUID) (MessageHandler, error)
    RequestStream(channelName string, payload interface{}) (MessageHandler, error)
    RequestStreamForDestination(channelName string, payload interface{}, destId *uuid.UUID) (MessageHandler, error)
}

var once sync.Once
var busInstance EventBus

// Get a reference to the EventBus.
func GetBus() EventBus {
    once.Do(func() {
        bf := new(bifrostEventBus)
        bf.init()
        busInstance = bf
    })
    return busInstance
}

type bifrostEventBus struct {
    ChannelManager ChannelManager
    Id             uuid.UUID
}

func (bus *bifrostEventBus) GetId() *uuid.UUID {
    return &bus.Id
}

func (bus *bifrostEventBus) init() {

    bus.Id = uuid.New()
    bus.ChannelManager = new(busChannelManager)
    bus.ChannelManager.Boot()
    fmt.Printf("🌈 Bifröst booted with Id [%s]\n", bus.Id.String())
}

// Get a pointer to the ChannelManager for managing Channels.
func (bus *bifrostEventBus) GetChannelManager() ChannelManager {
    return bus.ChannelManager
}

// Send a Response type (inbound) message on Channel, with supplied Payload.
// Throws error if the Channel does not exist.
func (bus *bifrostEventBus) SendResponseMessage(channelName string, payload interface{}, destId *uuid.UUID) error {
    channelObject, err := bus.ChannelManager.GetChannel(channelName)
    if err != nil {
        return err
    }
    config := buildConfig(channelName, payload, destId)
    message := GenerateResponse(config)
    sendMessageToChannel(channelObject, message)
    return nil
}

// Send a Request type message (outbound) message on Channel, with supplied Payload.
// Throws error if the Channel does not exist.
func (bus *bifrostEventBus) SendRequestMessage(channelName string, payload interface{}, destId *uuid.UUID) error {
    channelObject, err := bus.ChannelManager.GetChannel(channelName)
    if err != nil {
        return err
    }
    config := buildConfig(channelName, payload, destId)
    message := GenerateRequest(config)
    sendMessageToChannel(channelObject, message)
    return nil
}

// Send a Error type message (outbound) message on Channel, with supplied error
// Throws error if the Channel does not exist.
func (bus *bifrostEventBus) SendErrorMessage(channelName string, err error, destId *uuid.UUID) error {
    channelObject, chanErr := bus.ChannelManager.GetChannel(channelName)
    if chanErr != nil {
        return err
    }

    config := buildError(channelName, err, destId)
    message := GenerateError(config)
    sendMessageToChannel(channelObject, message)
    return nil
}

// Listen to stream of Response (inbound) messages on Channel. Will keep on ticking until closed.
// Returns MessageHandler
//  // To close an open stream.
//  handler, Err := bus.ListenStream("my-Channel")
//  // ...
//  handler.close() // this will close the stream.
func (bus *bifrostEventBus) ListenStream(channelName string) (MessageHandler, error) {
    channel, err := getChannelFromManager(bus, channelName)
    if err != nil {
        return nil, err
    }
    messageHandler := bus.wrapMessageHandler(channel, Response, true, false, nil)
    return messageHandler, nil
}

// Listen to stream of Response (inbound) messages on Channel for a specific Destination. Will keep on ticking until closed.
// Returns MessageHandler
//  // To close an open stream.
//  handler, Err := bus.ListenStream("my-Channel")
//  // ...
//  handler.close() // this will close the stream.
func (bus *bifrostEventBus) ListenStreamForDestination(channelName string, destId *uuid.UUID) (MessageHandler, error) {
    channel, err := getChannelFromManager(bus, channelName)
    if err != nil {
        return nil, err
    }
    if destId == nil {
        return nil, fmt.Errorf("Destination cannot be nil")
    }
    messageHandler := bus.wrapMessageHandler(channel, Response, false, false, destId)
    return messageHandler, nil
}

// Listen to a stream of Request (outbound) messages on Channel. Will keep on ticking until closed.
// Returns MessageHandler
//  // To close an open stream.
//  handler, Err := bus.ListenRequestStream("my-Channel")
//  // ...
//  handler.close() // this will close the stream.
func (bus *bifrostEventBus) ListenRequestStream(channelName string) (MessageHandler, error) {
    channel, err := getChannelFromManager(bus, channelName)
    if err != nil {
        return nil, err
    }
    messageHandler := bus.wrapMessageHandler(channel, Request, true, false, nil)
    return messageHandler, nil
}

// Listen to a stream of Request (outbound) messages on Channel for a specific Destination. Will keep on ticking until closed.
// Returns MessageHandler
//  // To close an open stream.
//  handler, Err := bus.ListenRequestStream("my-Channel")
//  // ...
//  handler.close() // this will close the stream.
func (bus *bifrostEventBus) ListenRequestStreamForDestination(channelName string, destId *uuid.UUID) (MessageHandler, error) {
    channel, err := getChannelFromManager(bus, channelName)
    if err != nil {
        return nil, err
    }
    if destId == nil {
        return nil, fmt.Errorf("Destination cannot be nil")
    }
    messageHandler := bus.wrapMessageHandler(channel, Request, false, false, destId)
    return messageHandler, nil
}


// Listen for a single Request (outbound) messages on Channel. Handler is closed after a single event.
// Returns MessageHandler
func (bus *bifrostEventBus) ListenRequestOnce(channelName string) (MessageHandler, error) {
    channel, err := getChannelFromManager(bus, channelName)
    if err != nil {
        return nil, err
    }
    id := checkForSuppliedId(nil)
    messageHandler := bus.wrapMessageHandler(channel, Request, true, false, id)
    messageHandler.runOnce = true
    return messageHandler, nil
}

// Listen for a single Request (outbound) messages on Channel with a specific Destination. Handler is closed after a single event.
// Returns MessageHandler
func (bus *bifrostEventBus) ListenRequestOnceForDestination(channelName string, destId *uuid.UUID) (MessageHandler, error) {
    channel, err := getChannelFromManager(bus, channelName)
    if err != nil {
        return nil, err
    }
    if destId == nil {
        return nil, fmt.Errorf("Destination cannot be nil")
    }
    messageHandler := bus.wrapMessageHandler(channel, Request, false, false, destId)
    messageHandler.runOnce = true
    return messageHandler, nil
}

func (bus *bifrostEventBus) ListenFirehose(channelName string) (MessageHandler, error) {
    channel, err := getChannelFromManager(bus, channelName)
    if err != nil {
        return nil, err
    }
    messageHandler := bus.wrapMessageHandler(channel, Request, true, true, nil)
    return messageHandler, nil
}

// Will listen for a single Response message on the Channel before un-subscribing automatically.
func (bus *bifrostEventBus) ListenOnce(channelName string) (MessageHandler, error) {
    channel, err := getChannelFromManager(bus, channelName)
    if err != nil {
        return nil, err
    }
    id := checkForSuppliedId(nil)
    messageHandler := bus.wrapMessageHandler(channel, Response, true, false, id)
    messageHandler.runOnce = true
    return messageHandler, nil
}

// Will listen for a single Response message on the Channel before un-subscribing automatically.
func (bus *bifrostEventBus) ListenOnceForDestination(channelName string, destId *uuid.UUID) (MessageHandler, error) {
    channel, err := getChannelFromManager(bus, channelName)
    if err != nil {
        return nil, err
    }
    if destId == nil {
        return nil, fmt.Errorf("Destination cannot be nil")
    }
    messageHandler := bus.wrapMessageHandler(channel, Response, false, false, destId)
    messageHandler.runOnce = true
    return messageHandler, nil
}

// Send a request message with Payload and wait for and Handle a single response message.
// Returns MessageHandler or error if the Channel is unknown
func (bus *bifrostEventBus) RequestOnce(channelName string, payload interface{}) (MessageHandler, error) {
    channel, err := getChannelFromManager(bus, channelName)
    if err != nil {
        return nil, err
    }
    destId := checkForSuppliedId(nil)
    messageHandler := bus.wrapMessageHandler(channel, Response, true, false, destId)
    config := buildConfig(channelName, payload, destId)
    message := GenerateRequest(config)
    messageHandler.requestMessage = message
    messageHandler.runOnce = true
    return messageHandler, nil
}

// Send a request message with Payload and wait for and Handle a single response message for a targeted Destination
// Returns MessageHandler or error if the Channel is unknown
func (bus *bifrostEventBus) RequestOnceForDestination(channelName string, payload interface{}, destId *uuid.UUID) (MessageHandler, error) {
    channel, err := getChannelFromManager(bus, channelName)
    if err != nil {
        return nil, err
    }
    if destId == nil {
        return nil, fmt.Errorf("Destination cannot be nil")
    }
    messageHandler := bus.wrapMessageHandler(channel, Response, false, false, destId)
    config := buildConfig(channelName, payload, destId)
    message := GenerateRequest(config)
    messageHandler.requestMessage = message
    messageHandler.runOnce = true
    return messageHandler, nil
}

func getChannelFromManager(bus *bifrostEventBus, channelName string) (*Channel, error) {
    channelManager := bus.ChannelManager
    channel, err := channelManager.GetChannel(channelName)
    return channel, err
}

// Send a request message with Payload and wait for and Handle all response messages.
// Returns MessageHandler or error if Channel is unknown
func (bus *bifrostEventBus) RequestStream(channelName string, payload interface{}) (MessageHandler, error) {
    channel, err := getChannelFromManager(bus, channelName)
    if err != nil {
        return nil, err
    }
    id := checkForSuppliedId(nil)
    messageHandler := bus.wrapMessageHandler(channel, Response, true, false, id)
    config := buildConfig(channelName, payload, id)
    message := GenerateRequest(config)
    messageHandler.requestMessage = message
    messageHandler.runOnce = false
    return messageHandler, nil
}

// Send a request message with Payload and wait for and Handle all response messages with a supplied Destination
// Returns MessageHandler or error if Channel is unknown
func (bus *bifrostEventBus) RequestStreamForDestination(channelName string, payload interface{}, destId *uuid.UUID) (MessageHandler, error) {
    channel, err := getChannelFromManager(bus, channelName)
    if err != nil {
        return nil, err
    }
    if destId == nil {
        return nil, fmt.Errorf("Destination cannot be nil")
    }
    messageHandler := bus.wrapMessageHandler(channel, Response, false, false, destId)
    config := buildConfig(channelName, payload, destId)
    message := GenerateRequest(config)
    messageHandler.requestMessage = message
    messageHandler.runOnce = false
    return messageHandler, nil
}


func checkForSuppliedId(id *uuid.UUID) *uuid.UUID {
    if id == nil {
        i := uuid.New()
        id = &i
    }
    return id
}

func checkHandlerHasRun(handler *messageHandler) bool {
    return handler.hasRun
}

func checkHandlerSingleRun(handler *messageHandler) bool {
    return handler.runOnce
}

func (bus *bifrostEventBus) wrapMessageHandler(channel *Channel, direction Direction, ignoreId bool, allTraffic bool, destId *uuid.UUID) *messageHandler {
    messageHandler := createMessageHandler(channel, destId)
    messageHandler.ignoreId = ignoreId
    errorHandler := func(err error) {
        if messageHandler.errorHandler != nil {
            if checkHandlerSingleRun(messageHandler) {
                if !checkHandlerHasRun(messageHandler) {
                    messageHandler.hasRun = true
                    messageHandler.runCount++
                    messageHandler.errorHandler(err)
                }
            } else {
                messageHandler.hasRun = true
                messageHandler.errorHandler(err)
            }
        }
    }
    successHandler := func(msg *Message) {
        if messageHandler.successHandler != nil {
            if checkHandlerSingleRun(messageHandler) {
                if !checkHandlerHasRun(messageHandler) {
                    messageHandler.hasRun = true
                    messageHandler.runCount++
                    messageHandler.successHandler(msg)
                }
            } else {
                messageHandler.hasRun = true
                messageHandler.runCount++
                messageHandler.successHandler(msg)
            }
        }
    }

    handlerWrapper := func(msg *Message) {
        dir := direction
        id := messageHandler.destination
        if allTraffic {
            if msg.Direction == Error {
                errorHandler(msg.Error)
            } else {
                successHandler(msg)
            }
        } else {
            if msg.Direction == dir {
                // if we're checking for specific traffic, check a Destination match is required.
                if !messageHandler.ignoreId && (msg.DestinationId != nil && id != nil) && (id.ID() == msg.DestinationId.ID()) {
                    successHandler(msg)
                }
                if messageHandler.ignoreId {
                    successHandler(msg)
                }
            }
            if msg.Direction == Error {
                errorHandler(msg.Error)
            }
        }
    }

    messageHandler.wrapperFunction = handlerWrapper
    return messageHandler
}

func sendMessageToChannel(channelObject *Channel, message *Message) {
    channelObject.Send(message)
}

func buildConfig(channelName string, payload interface{}, destinationId *uuid.UUID) *MessageConfig {
    config := new(MessageConfig)
    id := uuid.New()
    config.Id = &id
    config.Destination = destinationId
    config.Channel = channelName
    config.Payload = payload
    return config
}

func buildError(channelName string, err error, destinationId *uuid.UUID) *MessageConfig {
    config := new(MessageConfig)
    id := uuid.New()
    config.Id = &id
    config.Destination = destinationId
    config.Channel = channelName
    config.Err = err
    return config
}

func createMessageHandler(channel *Channel, destinationId *uuid.UUID) *messageHandler {
    messageHandler := new(messageHandler)
    messageHandler.channel = channel
    id := uuid.New()
    messageHandler.id = &id
    messageHandler.destination = destinationId
    return messageHandler
}
