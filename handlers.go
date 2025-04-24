package jsonrpc2

import (
	"context"
)

type NotificationHandler interface {
	Notify(context.Context, Parameters)
}

type NotificationHandlerFunc func(context.Context, Parameters)

type notficationHandlerFuncWrapper struct {
	handlerFunc NotificationHandlerFunc
}

func (h *notficationHandlerFuncWrapper) Notify(ctx context.Context, params Parameters) {
	h.handlerFunc(ctx, params)
}

var _ NotificationHandler = &notficationHandlerFuncWrapper{}

type CallHandler interface {
	Handle(context.Context, Parameters) (any, *JsonRpcError)
}

// the second argument is the parameters,
type CallHandlerFunc func(context.Context, Parameters) (any, *JsonRpcError)

type callHandlerFuncWrapper struct {
	handlerFunc CallHandlerFunc
}

func (h *callHandlerFuncWrapper) Handle(ctx context.Context, params Parameters) (any, *JsonRpcError) {
	r, err := h.handlerFunc(ctx, params)
	if err != nil {
		return nil, err
	}
	return r, nil
}

var _ CallHandler = &callHandlerFuncWrapper{}

type CancelHandler interface {
	Handle(context.Context, Parameters) (any, *JsonRpcError)
}

type CancelHandlerFunc func(context.Context, Parameters) (any, *JsonRpcError)

type cancelHandlerFuncWrapper struct {
	handlerFunc CancelHandlerFunc
}

func (h *cancelHandlerFuncWrapper) Handle(ctx context.Context, params Parameters) (any, *JsonRpcError) {
	response, err := h.handlerFunc(ctx, params)
	if err != nil {
		return nil, err
	}
	return response, nil
}

var _ CancelHandler = &cancelHandlerFuncWrapper{}
