package jsonrpc2

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
)

// This is how users will get the results data out.
type ClientResponse interface {
	fromAny(r any) error
}

type Client interface {
	Call(context.Context, string, any) (any, error)
	Notify(context.Context, string, any) error
	CallAsync(context.Context, string, any) (chan any, *string, error)
	Cancel(context.Context, string, any, string) error
}

type DefaultClient struct {
	conn *Connection
	log  *slog.Logger

	seq *atomic.Uint64
}

func NewDefaultClient(conn *Connection, log *slog.Logger) (Client, error) {
	return &DefaultClient{
		conn: conn,
		log:  log,
		seq:  &atomic.Uint64{},
	}, nil
}

var _ Client = &DefaultClient{}

func (c *DefaultClient) getID() *string {
	s := fmt.Sprintf("%v", c.seq.Add(1))
	return &s
}

func (c *DefaultClient) Call(ctx context.Context, method string, request any) (any, error) {
	returnChan, _, err := c.CallAsync(ctx, method, request)
	if err != nil {
		return nil, err
	}
	c.log.Log(ctx, slog.Level(-7), "waiting for response", "request", request)
	resp := <-returnChan
	return resp, err
}

func (c *DefaultClient) Notify(ctx context.Context, method string, request any) error {
	writeErrorChan := make(chan error)
	requestCtx := context.WithoutCancel(ctx)
	entry := writerEntry{
		ctx: requestCtx,
		clientWriterEntry: clientWriterEntry{
			errorChan: writeErrorChan,
		},
	}
	switch v := request.(type) {
	case *Request:
		entry.request = v
	case Request:
		entry.request = &v
	case *BatchRequest:
		entry.batchRequest = *v
	case BatchRequest:
		entry.batchRequest = v
	default:
		entry.request = &Request{
			Version: "2.0",
			Method:  method,
			Params:  v,
		}
	}
	c.conn.writeEntry <- entry
	err := <-writeErrorChan
	if err != nil {
		return err
	}
	return err
}

func (c *DefaultClient) CallAsync(ctx context.Context, method string, request any) (chan any, *string, error) {
	writeErrorChan := make(chan error)
	responseChan := make(chan any)
	requestCtx := context.WithoutCancel(ctx)
	entry := writerEntry{
		ctx: requestCtx,
		clientWriterEntry: clientWriterEntry{
			errorChan:    writeErrorChan,
			responseChan: responseChan,
		},
	}
	var id *string
	switch v := request.(type) {
	case *Request:
		entry.request = v
		id = v.ID
	case Request:
		entry.request = &v
		id = v.ID
	case *BatchRequest:
		entry.batchRequest = *v
	case BatchRequest:
		entry.batchRequest = v
	default:
		entry.request = &Request{
			Version: "2.0",
			Method:  method,
			Params:  v,
			ID:      c.getID(),
		}
		id = entry.request.ID
	}
	c.conn.writeEntry <- entry
	err := <-writeErrorChan
	if err != nil {
		return nil, nil, err
	}
	return responseChan, id, nil
}

// TODO: Some things require a response for cancel, we will need to add this.
func (c *DefaultClient) Cancel(ctx context.Context, method string, request any, id string) error {
	writeErrorChan := make(chan error)
	requestCtx := context.WithoutCancel(ctx)
	entry := writerEntry{
		ctx: requestCtx,
		clientWriterEntry: clientWriterEntry{
			errorChan: writeErrorChan,
			cancelID:  id,
		},
	}
	switch v := request.(type) {
	case *Request:
		entry.cancelRequest = v
	default:
		entry.cancelRequest = &Request{
			Version: "2.0",
			Method:  method,
			Params:  v,
		}
	}
	c.conn.writeEntry <- entry
	err := <-writeErrorChan
	if err != nil {
		return err
	}
	return nil
}
