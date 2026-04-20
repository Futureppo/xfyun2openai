package api

import (
	"net/http"

	"xfyun2openai/internal/openai"
)

func (s *Service) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (s *Service) handleModels(w http.ResponseWriter, _ *http.Request) {
	items := make([]openai.ModelItem, 0, len(s.cfg.Models))
	for _, name := range s.cfg.SortedModelNames() {
		items = append(items, openai.ModelItem{
			ID:      name,
			Object:  "model",
			Created: 0,
			OwnedBy: "xfyun",
		})
	}

	writeJSON(w, http.StatusOK, openai.ModelListResponse{
		Object: "list",
		Data:   items,
	})
}
