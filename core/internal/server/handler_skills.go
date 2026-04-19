package server

import (
	"net/http"

	"biene/internal/skills"
)

type skillResponse struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Dir          string `json:"dir"`
	FilePath     string `json:"file_path"`
	Instructions string `json:"instructions"`
}

type skillsCatalogResponse struct {
	Root   string          `json:"root"`
	Skills []skillResponse `json:"skills"`
}

// handleListSkills returns the global skills catalog under ~/.biene/skills.
// GET /api/skills
func (s *Server) handleListSkills(w http.ResponseWriter, _ *http.Request) {
	metas, root, err := skills.ScanGlobal()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]skillResponse, 0, len(metas))
	for _, meta := range metas {
		def, err := skills.LoadDefinition(meta)
		if err != nil {
			continue
		}
		items = append(items, skillResponse{
			Name:         def.Name,
			Description:  def.Description,
			Dir:          def.Dir,
			FilePath:     def.FilePath,
			Instructions: def.Instructions,
		})
	}

	writeJSON(w, http.StatusOK, skillsCatalogResponse{
		Root:   root,
		Skills: items,
	})
}
