package httptransport

import (
	"encoding/json"
	"net/http"

	"smart-meeting-notes/internal/app/usecase"
)

func NewRouter(pingSvc *usecase.PingService) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		msg, err := pingSvc.Ping(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": msg,
		})
	})

	return mux
}
