package websocket

import "github.com/go-pkgz/routegroup"

func Mount(api *routegroup.Bundle, hub *Hub, validateSession SessionValidator, allowedOrigin string) {
	api.HandleFunc("GET /ws", NewHandler(hub, validateSession, allowedOrigin))
}
