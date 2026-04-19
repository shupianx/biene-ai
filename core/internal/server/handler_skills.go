package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"biene/internal/skills"
)

type skillResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Instructions string `json:"instructions"`
}

type skillsCatalogResponse struct {
	Root                   string          `json:"root"`
	Skills                 []skillResponse `json:"skills"`
	DefaultEnabledSkillIDs []string        `json:"default_enabled_skill_ids"`
}

type updateSkillsConfigRequest struct {
	DefaultEnabledSkillIDs []string `json:"default_enabled_skill_ids"`
}

// handleListSkills returns the skill repository catalog under ~/.biene/skills.
// GET /api/skills
func (s *Server) handleListSkills(w http.ResponseWriter, _ *http.Request) {
	resp, err := loadSkillsCatalogResponse()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleUpdateSkillsConfig updates the default-enabled skill repository list.
// POST /api/skills/config
func (s *Server) handleUpdateSkillsConfig(w http.ResponseWriter, r *http.Request) {
	var req updateSkillsConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	normalized := make([]string, 0, len(req.DefaultEnabledSkillIDs))
	for _, id := range req.DefaultEnabledSkillIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		normalized = append(normalized, id)
	}

	if _, err := skills.SetRepositoryDefaultEnabledByID(normalized); err != nil {
		status := http.StatusInternalServerError
		if strings.HasPrefix(err.Error(), "skill not found:") {
			status = http.StatusBadRequest
		}
		writeError(w, status, err.Error())
		return
	}

	resp, err := loadSkillsCatalogResponse()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleImportSkills imports one uploaded skill repository folder.
// POST /api/skills/import
func (s *Server) handleImportSkills(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart upload")
		return
	}

	form := r.MultipartForm
	if form == nil || len(form.File["files"]) == 0 {
		writeError(w, http.StatusBadRequest, "no files uploaded")
		return
	}
	defer form.RemoveAll()

	uploadedFiles := make([]skills.UploadedFile, 0, len(form.File["files"]))
	for _, header := range form.File["files"] {
		header := header
		uploadedFiles = append(uploadedFiles, skills.UploadedFile{
			Path: header.Filename,
			Open: func() (io.ReadCloser, error) {
				return header.Open()
			},
		})
	}

	if _, err := skills.ImportRepositoryFiles(uploadedFiles); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := loadSkillsCatalogResponse()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleDeleteSkill deletes one skill repository entry by its stable id.
// DELETE /api/skills/{id}
func (s *Server) handleDeleteSkill(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "skill id is required")
		return
	}

	if _, err := skills.DeleteRepositorySkillByID(id); err != nil {
		status := http.StatusInternalServerError
		if strings.HasPrefix(err.Error(), "skill not found:") {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}

	resp, err := loadSkillsCatalogResponse()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func loadSkillsCatalogResponse() (skillsCatalogResponse, error) {
	metas, root, err := skills.ScanRepository()
	if err != nil {
		return skillsCatalogResponse{}, err
	}

	config, err := skills.LoadRepositoryConfig()
	if err != nil {
		return skillsCatalogResponse{}, err
	}

	defaultEnabled := make([]string, 0, len(config.DefaultEnabledSkillDirs))
	defaultSeen := make(map[string]struct{}, len(config.DefaultEnabledSkillDirs))
	for _, dir := range config.DefaultEnabledSkillDirs {
		id := skills.RepositorySkillID(root, dir)
		if id == "" {
			continue
		}
		if _, exists := defaultSeen[id]; exists {
			continue
		}
		defaultSeen[id] = struct{}{}
		defaultEnabled = append(defaultEnabled, id)
	}

	items := make([]skillResponse, 0, len(metas))
	for _, meta := range metas {
		def, err := skills.LoadDefinition(meta)
		if err != nil {
			continue
		}
		id := skills.RepositorySkillID(root, def.Dir)
		if id == "" {
			return skillsCatalogResponse{}, fmt.Errorf("cannot derive skill id for %s", def.Dir)
		}
		items = append(items, skillResponse{
			ID:           id,
			Name:         def.Name,
			Description:  def.Description,
			Instructions: def.Instructions,
		})
	}

	return skillsCatalogResponse{
		Root:                   root,
		Skills:                 items,
		DefaultEnabledSkillIDs: defaultEnabled,
	}, nil
}
