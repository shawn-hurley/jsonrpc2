package jsonrpc2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

type GetCancelID func(params Parameters) (string, error)

// ConnectionOptions holds the options for new connections.
type ConnectionOptions struct {
	// Framer allows control over the message framing and encoding.
	// If nil, HeaderFramer will be used.
	Framer Framer
	// Preempter allows registration of a pre-queue message handler.
	// If nil, no messages will be preempted.
	Preempter Preempter

	// Router is responsible for mapping the request methods to the correct handler
	Router Router
}

// idea is taken from golang's exp/jsonrpc2
// Preempter handles messages on a connection before they are queued to the main
// handler.
// Primarily this is used for cancel handlers or notifications for which out of
// order processing is not an issue.
type Preempter interface {
	// Preempt is invoked for each incoming request before it is queued.
	// If the request is a call, it must return a value or an error for the reply.
	// Preempt should not block or start any new messages on the connection.
	Preempt(ctx context.Context, req *Request) error
}

type PreempterChain struct {
	Preempters []Preempter
}

func (p *PreempterChain) Preempt(ctx context.Context, req *Request) error {
	for _, p := range p.Preempters {
		if err := p.Preempt(ctx, req); err != nil {
			if errors.As(err, &JsonRpcError{}) {
				return err
			}
		}
	}
	return nil
}

type clientWriterEntry struct {
	batchRequest  BatchRequest
	request       *Request
	cancelRequest *Request
	cancelID      string
	errorChan     chan error
	responseChan  chan any
	cancelFunc    context.CancelFunc
}

type serverWriterEntry struct {
	batchResponse BatchResponse
	response      *Response
	err           error
}

type writerEntry struct {
	ctx context.Context
	serverWriterEntry
	clientWriterEntry
}

type serverRequest struct {
	ctx        context.Context
	request    *Request
	handler    *handlerWrapper
	params     Parameters
	cancelFunc context.CancelFunc
}

type handlerWrapper struct {
	callHandler         CallHandler
	notificationHandler NotificationHandler
}

type clientRequest struct {
	ctx            context.Context
	request        *Request
	cancelFunc     context.CancelFunc
	batchRequestID *int64
	responseChan   chan any
}

func newHandleWrapper(notificationHandler NotificationHandler, callHandler CallHandler) *handlerWrapper {
	return &handlerWrapper{
		callHandler:         callHandler,
		notificationHandler: notificationHandler,
	}
}

func (h *handlerWrapper) Call(ctx context.Context, params Parameters) (any, *JsonRpcError) {
	if h.callHandler != nil {
		return h.callHandler.Handle(ctx, params)
	}
	if h.notificationHandler != nil {
		h.notificationHandler.Notify(ctx, params)
		return nil, nil
	}
	return nil, NewJsonRpcError(InternalError, nil)
}

// Connection manages the jsonrpc2 protocol, connecting responses back to their
// calls.
// Connection is bidirectional; it does not have a designated server or client
// end.
type Connection struct {
	seq *atomic.Int64
	log *slog.Logger

	conn           io.ReadWriteCloser
	requests       chan BatchRequest
	responseOutput chan BatchResponse
	response       chan BatchResponse
	writeEntry     chan writerEntry

	reader    Reader
	writer    Writer
	preempter Preempter
	router    Router

	ctx                      context.Context
	cancelFunc               context.CancelFunc
	serverRequestLock        *sync.RWMutex
	serverRequestsInProgress map[string]serverRequest
	clientRequestLock        *sync.RWMutex
	clientRequestsInProgress map[string]clientRequest
	// Look up for all pending responses in a batch request
	// should be controlled by the clientRequest lock
	batchRequestInProgress map[int64]map[string]struct{}
}

// Connection needs to have:
// 1. A routine to read new messages coming to the connection which should use the Framer.
// 2. A routine to Preempt the requests, and then pass them to the appriate handler if allowed.
// 3. A routine to push the results of the handler
func NewConnection(ctx context.Context, conn io.ReadWriteCloser, options ConnectionOptions, log *slog.Logger) (*Connection, error) {
	// Create a new context for this particular connection, with a parent of the passed in context.

	ctx, cancelFunc := context.WithCancel(ctx)
	c := &Connection{
		seq:  &atomic.Int64{},
		log:  log,
		conn: conn,
		// TODO: shawn-hurley consider making these buffered or make it a connection option
		requests:                 make(chan BatchRequest),
		responseOutput:           make(chan BatchResponse),
		response:                 make(chan BatchResponse),
		writeEntry:               make(chan writerEntry),
		reader:                   options.Framer.Reader(conn),
		writer:                   options.Framer.Writer(conn),
		preempter:                options.Preempter,
		router:                   options.Router,
		ctx:                      ctx,
		cancelFunc:               cancelFunc,
		serverRequestLock:        &sync.RWMutex{},
		serverRequestsInProgress: map[string]serverRequest{},
		clientRequestLock:        &sync.RWMutex{},
		clientRequestsInProgress: map[string]clientRequest{},
		batchRequestInProgress:   map[int64]map[string]struct{}{},
	}

	go c.readFromConn(log.With("name", "read-from-conn"))
	go c.handleRequests(log.With("name", "handle-request"))
	go c.handleResponse(log.With("name", "handle-resposne"))
	go c.write(log.With("name", "write-to-connn"))

	return c, nil
}

// readFromConn will be responsible for reading from the context, using the Framer to get the requests.
// the framer
func (c *Connection) readFromConn(log *slog.Logger) {
	for {
		message, bytesRead, err := c.reader.Read(c.ctx)
		if err != nil {
			// If error is of type JsonRPCError, then we need to respond with that,
			if errors.As(err, &JsonRpcError{}) {
				// TODO: send to write channel
				c.writeEntry <- writerEntry{
					ctx: c.ctx,
					serverWriterEntry: serverWriterEntry{
						err: err,
					},
				}
				continue
			}
			// If error is of type io.EOF or similar, then connection is closed and we should stop
			if err == io.EOF {
				log.Log(c.ctx, slog.Level(-5), "connection stopped", "err", err)
				return
			}
		}
		log.Log(c.ctx, slog.Level(-6), "read from connection", "err", err, "message", message, "bytes", bytesRead)
		if message == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		// Now we need to determine what type of message and where to put it.
		switch m := message.(type) {
		case *Request:
			// Send to request handler channel
			c.requests <- BatchRequest([]*Request{m})
		case *Response:
			// Send to response handler channel
			c.response <- BatchResponse([]*Response{m})
		case BatchRequest:
			c.requests <- m
		case BatchResponse:
			c.response <- m
		default:
			log.Log(c.ctx, slog.Level(-6), "unable to understand type from reader", "message", m)
			c.writeEntry <- writerEntry{
				ctx: c.ctx,
				serverWriterEntry: serverWriterEntry{
					err: NewJsonRpcError(InvalidRequest, nil),
				},
			}
			continue
		}
	}
}

// runs the handler to handle the request. In this case if returnChan is nil, it will not respond
// Allows for the cancelling of the request by context on the serverRequest.
func (c *Connection) runRequest(s serverRequest, returnChan chan writerEntry) {
	defer func() {
		if s.request.ID != nil {
			c.serverRequestLock.Lock()
			defer c.serverRequestLock.Unlock()
			delete(c.serverRequestsInProgress, *s.request.ID)
		}
	}()

	select {
	case <-s.ctx.Done():
		return
	default:
		responseData, rerr := s.handler.Call(s.ctx, s.params)
		if s.ctx.Err() != nil {
			return
		}
		if returnChan == nil {
			return
		}
		returnChan <- writerEntry{
			serverWriterEntry: serverWriterEntry{
				response: &Response{
					Version: "2.0",
					Result:  responseData,
					Error:   rerr,
					ID:      s.request.ID,
				},
			},
			ctx: s.ctx,
		}
	}
}

// hanndleRequests is responsbile for determing what type of request, and handeling it appropriatly.
// If a cancel request we will look up the request and cancel it.
// If it is a call request, we will find the right handler and allow it to start processing asyncrounously
// If it is a notification, we will find the right handler and allow it to start processing asyncrounsly
// Of note, this function is not async, it access the map of id to server requests in a controlled mannerf.
// Running of the request is done in runRequest.
func (c *Connection) handleRequest(ctx context.Context, log *slog.Logger, r *Request, returnChan chan writerEntry) *JsonRpcError {
	if cancelHandler, ok := c.router.IsCancelRequest(r.Method); ok {
		p, rerr := getParameters(cancelHandler.Parameters, r.Params, log)
		if rerr != nil {
			return rerr
		}
		id, err := cancelHandler.getCancelID(p)
		if err != nil {
			return NewJsonRpcError(InternalError, nil)
		}
		c.serverRequestLock.RLock()
		s, ok := c.serverRequestsInProgress[id]
		c.serverRequestLock.RUnlock()
		if !ok {
			c.log.Debug("no request to cancel")
			return nil
		}
		s.cancelFunc()
		return nil
	}
	if r.ID != nil {
		c.serverRequestLock.RLock()
		if _, ok := c.serverRequestsInProgress[*r.ID]; ok {
			c.serverRequestLock.RUnlock()
			return NewJsonRpcError(InvalidRequest, r)
		}
		c.serverRequestLock.RUnlock()
		// If a notification, we don't need to create a sequence
		h, err := c.router.RouteCall(r.Method)
		if err != nil {
			// Send error to response writer
			if rerr, ok := err.(*JsonRpcError); ok {
				return rerr
			} else {
				return NewJsonRpcError(InternalError, nil)
			}
		}
		p, rerr := getParameters(h.p, r.Params, log)
		if rerr != nil {
			return rerr
		}
		requestCtx, requestCtxCancel := context.WithCancel(ctx)

		// Here we handle the request
		s := serverRequest{
			ctx:        requestCtx,
			request:    r,
			handler:    newHandleWrapper(nil, h.h),
			params:     p,
			cancelFunc: requestCtxCancel,
		}
		c.serverRequestLock.Lock()
		c.serverRequestsInProgress[*r.ID] = s
		c.serverRequestLock.Unlock()

		go c.runRequest(s, returnChan)
		return nil
	} else {
		// If a notification, we don't need to create a sequence
		h, err := c.router.RouteNotifaction(r.Method)
		if err != nil {
			// Send error to response writer
			if rerr, ok := err.(*JsonRpcError); ok {
				return rerr
			} else {
				return NewJsonRpcError(InternalError, nil)
			}
		}
		p, rerr := getParameters(h.p, r.Params, log)
		if rerr != nil {
			return rerr
		}

		ctx := context.WithoutCancel(c.ctx)
		go c.runRequest(serverRequest{
			ctx:        ctx,
			request:    r,
			handler:    newHandleWrapper(h.h, nil),
			params:     p,
			cancelFunc: nil,
		}, nil)
		return nil
	}
}

// handleRequests is responsible for dealing with batching reqeusts
// when it is a single request, it will pass on handleRequest to take care of.
// If it is a batch request it will allow handleRequest to take of them asyncrounously
// It will be responsible for creating a batch response.
func (c *Connection) handleRequests(log *slog.Logger) {
	for {
		select {
		case batchRequest := <-c.requests:
			log.Debug("got request", "request", batchRequest)
			// Make a context with cancel for this requst
			if len(batchRequest) == 1 {
				rerr := c.handleRequest(c.ctx, log, batchRequest[0], c.writeEntry)
				if rerr != nil {
					c.log.Error("unable to handle request", "request", batchRequest[0], "error", rerr)
					c.writeEntry <- writerEntry{
						serverWriterEntry: serverWriterEntry{err: rerr},
					}
				}
			} else {
				batchResponse := BatchResponse{}
				batchResponseChan := make(chan writerEntry, len(batchRequest))
				batchCtx, cancel := context.WithCancel(c.ctx)
				defer cancel()
				for _, r := range batchRequest {
					if r.ID == nil {
						c.writeEntry <- writerEntry{serverWriterEntry: serverWriterEntry{err: NewJsonRpcError(InvalidRequest, nil)}}
						cancel()
						break
					}
					// Handle batch requests concurrently.
					// The Response objects being returned from a batch call MAY be returned in any order within the Array.
					err := c.handleRequest(batchCtx, log, r, batchResponseChan)
					if err != nil {
						c.log.Error("unable to handle request", "request", batchRequest[0], "error", err)
						batchResponseChan <- writerEntry{
							serverWriterEntry: serverWriterEntry{
								response: &Response{
									Version: "2.0",
									Result:  nil,
									Error:   err,
									ID:      r.ID,
								},
							},
						}
					}
				}
				for range len(batchRequest) {
					writeEntry := <-batchResponseChan
					c.log.Info("here", "w", writeEntry)
					if writeEntry.response != nil {
						batchResponse = append(batchResponse, writeEntry.response)
					}
				}
				if len(batchResponse) > 0 {
					c.writeEntry <- writerEntry{
						serverWriterEntry: serverWriterEntry{
							batchResponse: batchResponse,
						},
					}
				}
			}
			// Determine if call, or if notification
			// determine if we have a route, or respond with error
		case <-c.ctx.Done():
			c.log.Log(c.ctx, slog.Level(-6), "stopping handle of requests")
			return
		}
	}
}

func (c *Connection) handleResponse(log *slog.Logger) {
	for {
		select {
		case r := <-c.response:
			// Determine if we have a Call or Async call pending for this response
			// send response to that call.
			// we will need to handle matching requests
			var batchRequestID *int64
			var requestIDs map[string]struct{}
			var responseBatch []any
			var response any
			var responseChan chan any
			for i, resp := range r {
				if resp.ID == nil {
					log.Log(c.ctx, slog.Level(-6), "got a response with no id", "response", resp)
					break
				}
				c.clientRequestLock.RLock()
				clientRequest, ok := c.clientRequestsInProgress[*resp.ID]
				c.clientRequestLock.RUnlock()
				if !ok {
					log.Log(c.ctx, slog.Level(-6), "got a response without a matching request", "resposne", resp)
					break
				}
				log.Log(c.ctx, slog.Level(-7), "print clientRequest", "cr", fmt.Sprintf("%+v", clientRequest))
				if i == 0 {
					batchRequestID = clientRequest.batchRequestID
					responseChan = clientRequest.responseChan
					if responseChan == nil {
						log.Log(c.ctx, slog.Level(-6), "no place to send the resposne")
						break
					}
					if batchRequestID != nil {
						c.clientRequestLock.RLock()
						requestIDs, ok = c.batchRequestInProgress[*batchRequestID]
						c.clientRequestLock.RUnlock()
						if !ok {
							log.Log(c.ctx, slog.Level(-6), "got a response without a matching request", "resposne", resp)
							break
						}
						responseBatch = []any{}
					}
				}
				if batchRequestID != nil && requestIDs != nil {
					// verify the resp is in the batch
					if _, ok := requestIDs[*resp.ID]; !ok {
						log.Log(c.ctx, slog.Level(-6), "got a response with a invalid request in the batch", "resposne", resp)
						break
					}
					if resp.Result != nil {
						responseBatch = append(responseBatch, resp)
					} else {
						responseBatch = append(responseBatch, resp)
					}
				} else {
					if len(r) != 1 {
						log.Log(c.ctx, slog.Level(-6), "got a response with a invalid request in the batch", "resposne", resp)
						break
					}
					if resp.Result != nil {
						response = resp.Result
					} else {
						response = resp.Error
					}
				}
			}

			if responseChan != nil {
				if responseBatch != nil {
					if len(responseBatch) != len(requestIDs) {
						log.Log(c.ctx, slog.Level(-6), "got a response that did not fully match the batch request")
						continue
					}
					responseChan <- responseBatch
				} else {
					log.Log(c.ctx, slog.Level(-7), "responding with response", "response", response)
					responseChan <- response
				}
			}
		case <-c.ctx.Done():
			log.Log(c.ctx, slog.Level(-6), "stopping handle of responses")
			return
		}
	}
}

func (c *Connection) handleWriteError(err error, errorChan chan error) {
	errorChan <- err
	if err != nil {
		close(errorChan)
	}
}
func (c *Connection) write(log *slog.Logger) {
	for {
		select {
		case entry := <-c.writeEntry:
			if entry.request != nil {
				if entry.request.ID != nil {
					c.clientRequestLock.Lock()
					c.clientRequestsInProgress[*entry.request.ID] = clientRequest{
						ctx:          entry.ctx,
						request:      entry.request,
						cancelFunc:   entry.cancelFunc,
						responseChan: entry.responseChan,
					}
					c.clientRequestLock.Unlock()
				}
				bytesWriten, err := c.writer.Write(entry.ctx, entry.request)
				c.handleWriteError(err, entry.errorChan)
				if err != nil {
					log.Error("unable to write request", "request", entry.request, "error", err)
					close(entry.responseChan)
					// remove from InProgress
					c.clientRequestLock.Lock()
					delete(c.clientRequestsInProgress, *entry.request.ID)
					c.clientRequestLock.Unlock()
					continue
				}
				log.Log(entry.ctx, slog.Level(-6), "wrote request", "bytes", bytesWriten, "request", entry.request)
			}
			if entry.response != nil {
				bytesWriten, err := c.writer.Write(entry.ctx, entry.response)
				c.handleWriteError(err, entry.errorChan)
				if err != nil {
					log.Error("unable to write response", "response", entry.response, "error", err)
				}
				log.Log(entry.ctx, slog.Level(-6), "wrote response", "bytes", bytesWriten, "response", entry.response)
			}
			if entry.batchRequest != nil {
				batchRequestID := c.seq.Add(1)
				c.clientRequestLock.Lock()
				batchRequestIDs := map[string]struct{}{}
				for _, req := range entry.batchRequest {
					if req.ID != nil {
						clientRequestCtx, cfunc := context.WithCancel(entry.ctx)
						c.clientRequestsInProgress[*req.ID] = clientRequest{
							ctx:            clientRequestCtx,
							request:        req,
							cancelFunc:     cfunc,
							batchRequestID: &batchRequestID,
							responseChan:   entry.responseChan,
						}
						batchRequestIDs[*req.ID] = struct{}{}
					}
				}
				c.batchRequestInProgress[batchRequestID] = batchRequestIDs
				c.clientRequestLock.Unlock()
				bytesWriten, err := c.writer.Write(entry.ctx, entry.batchRequest)
				c.handleWriteError(err, entry.errorChan)
				if err != nil {
					log.Error("unable to write request", "batch request", entry.batchRequest, "error", err)
				}
				log.Log(entry.ctx, slog.Level(-6), "wrote request", "bytes", bytesWriten, "batch request", entry.batchRequest)
			}
			if entry.batchResponse != nil {
				bytesWriten, err := c.writer.Write(entry.ctx, entry.batchResponse)
				c.handleWriteError(err, entry.errorChan)
				if err != nil {
					log.Error("unable to write request", "request", entry.request, "error", err)
				}
				log.Log(entry.ctx, slog.Level(-6), "wrote request", "bytes", bytesWriten, "request", entry.request)

			}
			if entry.err != nil {
				var rpcError *JsonRpcError
				var ok bool
				if rpcError, ok = entry.err.(*JsonRpcError); !ok {
					// if this is not a json rpc error, then we need to convert it.
					rpcError = NewJsonRpcError(InternalError, entry.err.Error())
				}
				bytesWriten, err := c.writer.Write(entry.ctx, rpcError)
				c.handleWriteError(err, entry.errorChan)
				if err != nil {
					log.Error("unable to write err", "err", entry.err, "error", err)
				}
				log.Log(entry.ctx, slog.Level(-6), "wrote err", "bytes", bytesWriten, "err", entry.err)
			}
			if entry.cancelRequest != nil {
				if entry.cancelID == "" {
					err := fmt.Errorf("unable to cancel without id")
					c.handleWriteError(err, entry.errorChan)
				}
				c.clientRequestLock.Lock()
				delete(c.clientRequestsInProgress, *entry.request.ID)
				c.clientRequestLock.Unlock()
				bytesWriten, err := c.writer.Write(entry.ctx, entry.request)
				c.handleWriteError(err, entry.errorChan)
				log.Log(entry.ctx, slog.Level(-6), "wrote request", "bytes", bytesWriten, "request", entry.request)
			}
		case <-c.ctx.Done():
			log.Log(c.ctx, slog.LevelDebug, "finshed handling all writes")
			return
		}
	}
}
