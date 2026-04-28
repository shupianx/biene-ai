package server

import (
	"net/http"

	"biene/internal/templates"
)

// providerTemplatesResponse wraps the slice in an object so future
// metadata (versioning, "last updated", etc.) can be added without
// breaking client decoders.
type providerTemplatesResponse struct {
	Vendors []templates.Vendor `json:"vendors"`
}

// handleProviderTemplates serves the canonical model preset list.
// GET /api/provider-templates
func (s *Server) handleProviderTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, providerTemplatesResponse{Vendors: templates.Builtin})
}
