package rest

import (
	"net/http"

	// from project root:
	"svc-wallet/util/tracer"
)

// this middleware takes a request and generats a unique id and put it in the context
// we will use closue in this function
func RequestIDMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		reqId := tracer.GenerateRequestID() // create the id

		ctx := tracer.ContextWithRequestID(r.Context(), reqId) // create new context

		w.Header().Set("X-Request-ID", reqId) // attach the id to the response

		next.ServeHTTP(w, r.WithContext(ctx))
	}

}
