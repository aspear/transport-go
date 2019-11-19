// Copyright 2019 VMware, Inc. All rights reserved. -- VMware Confidential

package stompserver

import (
    "github.com/go-stomp/stomp/frame"
    "time"
)

type RawConnection interface {
    // Reads a single frame object
    ReadFrame() (*frame.Frame, error)
    // Sends a single frame object
    WriteFrame(frame *frame.Frame) error
    // Set deadline for reading frames
    SetReadDeadline(t time.Time)
    // Close the connection
    Close() error
}

type RawConnectionListener interface {
    // Blocks until a new RawConnection is established.
    Accept() (RawConnection, error)
    // Stops the connection listener.
    Close() error
}
