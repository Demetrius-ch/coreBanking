// handlers/audit.go
package handlers

import (
	"encoding/json"
	"net/http"

	"gestionbancaire/storage"
)

func GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	storage.Store.RLock()
	defer storage.Store.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(storage.Store.AuditLogs)
}
