// (c) Gareth Watts 2017
// Licensed under an MIT license
// See the LICENSE file for details

/*
Package adapter provides a method of connecting standard Go middleware handlers
to Gin's Engine.

The adapter uses the request context to pass Gin's context through the
wrapped handler and ensure the next middlware on the chain is called correctly.

Example of using it with the nosurf CSRF package

  engine.Use(adapter.Wrap(nosurf.NewPure))

If the middleware you're using doesn't comply with the f(http.Handler) http.Handler
interface, or if extra configuration of the middleware is required, use New
instead:

  nextHandler, wrapper := adapter.New()
  ns := nosurf.New(nextHandler)
  ns.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	  http.Error(w, "Custom error about invalid CSRF tokens", http.StatusBadRequest)
  }))
  engine.Use(wrapper(ns))
*/
package adapter

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// New returns a handler to be passed to middleware which will ensure
// the next handler in the chain is called correctly, along with
// a function that wraps the middleware's handler into a gin.HandlerFunc
// to be passed to Engine.Use.
//
// If the middleware does not call the handler it's wrapping, Abort is
// called on the Gin context.
func New() (http.Handler, func(h http.Handler) (func(c *gin.Context))) {
	nextHandler := new(connectHandler)
	makeGinHandler := func(h http.Handler) (func(c *gin.Context)) {
		return func(c *gin.Context) {
			state := &middlewareCtx{ctx: c}
			ctx := context.WithValue(c.Request.Context(), nextHandler, state)
			h.ServeHTTP(c.Writer, c.Request.WithContext(ctx))
			if !state.childCalled {
				c.Abort()
			}
		}
	}
	return nextHandler, makeGinHandler
}

// Wrap takes the common HTTP middleware function signature, calls it to generate
// a handler, and wraps it into a Gin middleware handler.
//
// This is just a convenience wrapper around New.
func Wrap(f func(h http.Handler) http.Handler) (func(c *gin.Context) {
	next, adapter := New()
	return adapter(f(next))
}

type connectHandler struct{}

// pull Gin's context from the request context and call the next item
// in the chain.
func (h *connectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	state := r.Context().Value(h).(*middlewareCtx)
	defer func(r *http.Request) { state.ctx.Request = r }(state.ctx.Request)
	state.ctx.Request = r
	state.childCalled = true
	state.ctx.Next()
}

type middlewareCtx struct {
	ctx         *gin.Context
	childCalled bool
}
