package routing

import "net/http"

type AuthMiddleware func(string, http.HandlerFunc) http.HandlerFunc
type AdminMiddleware func(http.HandlerFunc) http.HandlerFunc
