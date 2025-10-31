package respond

import (
	"encoding/json"
	"net/http"
)

func JSON(w http.ResponseWriter, r *http.Request, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}


func Error(w http.ResponseWriter, r *http.Request, code int, message string) {
	JSON(w, r, code, map[string]string{"error": message})
}
