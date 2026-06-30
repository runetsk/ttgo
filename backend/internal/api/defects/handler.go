package defects

import "ttgo/pkg/tracker/store"

type Handler struct{ store *store.Store }

func NewHandler(s *store.Store) *Handler { return &Handler{store: s} }
