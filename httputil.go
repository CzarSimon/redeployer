package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	requestIDHeader   = "X-Request-ID"
	tokenHeader       = "X-Deploy-Token"
	contentTypeHeader = "Content-Type"
)

var (
	errMethodNotAllowed = fmt.Errorf("Method not allowed")
	errUnauthorized     = fmt.Errorf("Unauthorized")
	errInternalError    = fmt.Errorf("Internal error")
	errBadRequest       = fmt.Errorf("Bad request")
)

// Context request context.
type Context struct {
	id    string
	start time.Time
	w     http.ResponseWriter
	r     *http.Request
	context.Context
}

func (ctx *Context) latency() string {
	duration := time.Now().Sub(ctx.start)
	return fmt.Sprintf("%.2f ms", float64(duration)/1e6)
}

func newContext(w http.ResponseWriter, r *http.Request) (*Context, error) {
	requestID := r.Header.Get(requestIDHeader)
	if requestID == "" {
		id, err := uuid.NewRandom()
		if err != nil {
			return nil, errInternalError
		}
		requestID = id.String()
	}

	return &Context{
		id:      requestID,
		start:   time.Now(),
		w:       w,
		r:       r,
		Context: context.Background(),
	}, nil
}

// Router wrapper around a http.ServeMux to provide
// authentication for specific routes, mathing a routes to http methods
// and wrapping HandlerFuncs with error handling an logging.
type router struct {
	mux     *http.ServeMux
	keyHash string
}

func newRouter(keyHash string) *router {
	return &router{
		mux:     http.NewServeMux(),
		keyHash: keyHash,
	}
}

func (router *router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	router.mux.ServeHTTP(w, r)
}

func (router *router) GET(pattern string, h handlerFunc, useAuth bool) {
	router.mux.Handle(pattern, newHandler(http.MethodGet, h, router.keyHash, useAuth))
}

func (router *router) POST(pattern string, h handlerFunc, useAuth bool) {
	router.mux.Handle(pattern, newHandler(http.MethodPost, h, router.keyHash, useAuth))
}

// HandlerFunc signature of a request handler.
type handlerFunc func(*Context) (int, error)

// Handler wrapper around a HandlerFunc to provide
// authentication, method checking, logging and error handling.
type handler struct {
	method  string
	handle  handlerFunc
	keyHash string
	useAuth bool
}

// NewHandler creates and returns a new Handler.
func newHandler(method string, h handlerFunc, keyHash string, useAuth bool) *handler {
	return &handler{
		method:  method,
		handle:  h,
		useAuth: useAuth,
		keyHash: keyHash,
	}
}

// ServeHTTP wrapps the call to the handlers HandlerFunc.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	ctx, err := newContext(w, r)
	if err != nil {
		ctx.sendError(err, http.StatusInternalServerError)
		return
	}

	logIncommingRequest(ctx)
	defer func() {
		logOutgoingRequest(ctx, status)
	}()

	err = assertMehthod(ctx, h.method, w)
	if err != nil {
		status = http.StatusMethodNotAllowed
		return
	}

	err = h.authenticate(r)
	if err != nil {
		status = http.StatusUnauthorized
		ctx.sendError(err, http.StatusUnauthorized)
		return
	}

	status, err = h.handle(ctx)

	if err != nil {
		ctx.sendError(err, status)
	}
}

func (h *handler) authenticate(r *http.Request) error {
	if !h.useAuth {
		return nil
	}

	token := r.Header.Get(tokenHeader)
	if h.keyHash != fmt.Sprintf("%x", sha256.Sum256([]byte(token))) {
		return errUnauthorized
	}

	return nil
}

func logIncommingRequest(ctx *Context) {
	message := fmt.Sprintf("Incomming request: %s %s", ctx.r.Method, ctx.r.URL.Path)
	log.Debugw(message, "requestId", ctx.id)
}

func logOutgoingRequest(ctx *Context, status int) {
	log.Debugw("Reqeust complete", "status", status, "latency", ctx.latency(), "requestId", ctx.id)
}

func assertMehthod(ctx *Context, method string, w http.ResponseWriter) error {
	if method != ctx.r.Method {
		ctx.sendError(errMethodNotAllowed, http.StatusMethodNotAllowed)
		return errMethodNotAllowed
	}

	return nil
}

func (ctx *Context) sendJSON(v interface{}) (int, error) {
	r, err := json.Marshal(v)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	ctx.w.Header().Set(requestIDHeader, ctx.id)
	ctx.w.Header().Set(contentTypeHeader, "application/json")
	ctx.w.WriteHeader(http.StatusOK)
	ctx.w.Write(r)
	return http.StatusOK, nil
}

func (ctx *Context) sendError(err error, status int) {
	r, _ := json.Marshal(ResponseMessage{
		Message: err.Error(),
	})

	ctx.w.Header().Set(requestIDHeader, ctx.id)
	ctx.w.Header().Set(contentTypeHeader, "application/json")
	ctx.w.WriteHeader(status)
	ctx.w.Write(r)
}

func (ctx *Context) sendOK() {
	r, _ := json.Marshal(ResponseMessage{
		Message: "OK",
	})

	ctx.w.Header().Set(requestIDHeader, ctx.id)
	ctx.w.Header().Set(contentTypeHeader, "application/json")
	ctx.w.WriteHeader(http.StatusOK)
	ctx.w.Write(r)
}
