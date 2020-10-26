package srv

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
)

type apiError struct {
	Message string `json:"omitempty"`
}

func (e apiError) write(w http.ResponseWriter, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	body, err := json.Marshal(e)
	if err != nil {
		log.Errorf("marshalling api error: %s", err)
		return
	}
	w.Write(body)
}

func writeError500(w http.ResponseWriter) {
	apiError{"internal server error"}.write(w, 500)
}
