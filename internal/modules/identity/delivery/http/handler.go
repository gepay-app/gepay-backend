package http

import "gepay/internal/modules/identity/domain"

type Handler struct {
	service domain.Service
}

func NewHandler(service domain.Service) *Handler {
	return &Handler{service}
}
