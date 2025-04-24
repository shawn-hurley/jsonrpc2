package jsonrpc2

import (
	"context"
	"io"
)

// Reader abstracts the transport mechanics from the JSON RPC protocol.
// A Conn reads messages from the reader it was provided on construction,
// and assumes that each call to Read fully transfers a single message,
// or returns an error.
// A reader is not safe for concurrent use, it is expected it will be used by
// a single Conn in a safe manner.
type Reader interface {
	// Read gets the next message from the stream.
	Read(context.Context) (MessageMap, int64, error)
}

// Writer abstracts the transport mechanics from the JSON RPC protocol.
// A Conn writes messages using the writer it was provided on construction,
// and assumes that each call to Write fully transfers a single message,
// or returns an error.
// A writer is not safe for concurrent use, it is expected it will be used by
// a single Conn in a safe manner.
type Writer interface {
	// Write sends a message to the stream.
	Write(context.Context, MessageMap) (int64, error)
}

// Framer wraps low level byte readers and writers into jsonrpc2 message
// readers and writers.
// It is responsible for the framing and encoding of messages into wire form.
type Framer interface {
	Reader(r io.Reader) Reader
	Writer(w io.Writer) Writer
}
