package main

import (
	"context"
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
	errBadRequest       = fmt.Errorf("Bad request")        // 400
	errUnauthorized     = fmt.Errorf("Unauthorized")       // 401
	errForbidden        = fmt.Errorf("Forbidden")          // 403
	errNotFound         = fmt.Errorf("Not found")          // 404
	errMethodNotAllowed = fmt.Errorf("Method not allowed") // 405
	errInternalError    = fmt.Errorf("Internal error")     // 500
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
	authKey scryptKey
}

func newRouter(authKey AuthKey) *router {
	key, err := parseKey(authKey.Key)
	if err != nil {
		log.Fatalw("Failed to parse key", "key", authKey.Key, "")
	}
	key.salt = authKey.Salt

	return &router{
		mux:     http.NewServeMux(),
		authKey: key,
	}
}

func (router *router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	router.mux.ServeHTTP(w, r)
}

func (router *router) GET(pattern string, h handlerFunc, useAuth bool) {
	router.mux.Handle(pattern, newHandler(http.MethodGet, h, router.authKey, useAuth))
}

func (router *router) POST(pattern string, h handlerFunc, useAuth bool) {
	router.mux.Handle(pattern, newHandler(http.MethodPost, h, router.authKey, useAuth))
}

// HandlerFunc signature of a request handler.
type handlerFunc func(*Context) (int, error)

// Handler wrapper around a HandlerFunc to provide
// authentication, method checking, logging and error handling.
type handler struct {
	method  string
	handle  handlerFunc
	authKey scryptKey
	useAuth bool
}

// NewHandler creates and returns a new Handler.
func newHandler(method string, h handlerFunc, key scryptKey, useAuth bool) *handler {
	return &handler{
		method:  method,
		handle:  h,
		useAuth: useAuth,
		authKey: key,
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
	defer recoverFromPanic(ctx, "handler.ServeHTTP", true)

	logIncommingRequest(ctx)

	err = assertMehthod(ctx, h.method, w)
	if err != nil {
		status = http.StatusMethodNotAllowed
		logOutgoingRequest(ctx, status)
		return
	}

	err = h.authenticate(r)
	if err != nil {
		status = http.StatusUnauthorized
		ctx.sendError(err, http.StatusUnauthorized)
		logOutgoingRequest(ctx, status)
		return
	}

	status, err = h.handle(ctx)

	if err != nil {
		ctx.sendError(err, status)
	}
	logOutgoingRequest(ctx, status)
}

func (h *handler) authenticate(r *http.Request) error {
	if !h.useAuth {
		return nil
	}

	token := r.Header.Get(tokenHeader)
	hash, err := deriveKey(token, h.authKey)
	if err != nil {
		return errInternalError
	}

	if h.authKey.hash != hash {
		return errUnauthorized
	}

	return nil
}

func logIncommingRequest(ctx *Context) {
	message := fmt.Sprintf("Incomming request: %s %s", ctx.r.Method, ctx.r.URL.Path)
	log.Debugw(message, "requestId", ctx.id)
}

func logOutgoingRequest(ctx *Context, status int) {
	log.Debugw("Request complete", "status", status, "latency", ctx.latency(), "requestId", ctx.id)
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

func recoverFromPanic(ctx *Context, event string, sendError bool) {
	if r := recover(); r != nil {
		err, ok := r.(error)
		if !ok {
			err = fmt.Errorf("[recover] Cause: %v", r)
		}

		log.Errorw(event+" caused a panic", "error", err, "requestId", ctx.id)
		if sendError {
			ctx.sendError(errInternalError, http.StatusInternalServerError)
		}
	}
}
