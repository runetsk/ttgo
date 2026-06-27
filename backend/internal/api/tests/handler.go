package tests

import (
	"ttgo/pkg/tracker/store"

	"github.com/microcosm-cc/bluemonday"
)

type Handler struct {
	store     *store.Store
	sanitizer *bluemonday.Policy
}

func NewHandler(s *store.Store, sanitizer *bluemonday.Policy) *Handler {
	return &Handler{
		store:     s,
		sanitizer: sanitizer,
	}
}
