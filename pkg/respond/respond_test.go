package respond

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSON(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		data     interface{}
		wantCode int
		wantBody map[string]interface{}
	}{
		{
			name:     "success response",
			code:     http.StatusOK,
			data:     map[string]string{"message": "success"},
			wantCode: http.StatusOK,
			wantBody: map[string]interface{}{"message": "success"},
		},
		{
			name:     "created response",
			code:     http.StatusCreated,
			data:     map[string]int{"id": 123},
			wantCode: http.StatusCreated,
			wantBody: map[string]interface{}{"id": float64(123)}, // JSON unmarshals numbers as float64
		},
		{
			name:     "empty object",
			code:     http.StatusOK,
			data:     map[string]string{},
			wantCode: http.StatusOK,
			wantBody: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			JSON(w, r, tt.code, tt.data)

			assert.Equal(t, tt.wantCode, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var got map[string]interface{}
			err := json.NewDecoder(w.Body).Decode(&got)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBody, got)
		})
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		message  string
		wantCode int
		wantErr  string
	}{
		{
			name:     "bad request",
			code:     http.StatusBadRequest,
			message:  "invalid input",
			wantCode: http.StatusBadRequest,
			wantErr:  "invalid input",
		},
		{
			name:     "not found",
			code:     http.StatusNotFound,
			message:  "resource not found",
			wantCode: http.StatusNotFound,
			wantErr:  "resource not found",
		},
		{
			name:     "internal error",
			code:     http.StatusInternalServerError,
			message:  "something went wrong",
			wantCode: http.StatusInternalServerError,
			wantErr:  "something went wrong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			Error(w, r, tt.code, tt.message)

			assert.Equal(t, tt.wantCode, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var got map[string]string
			err := json.NewDecoder(w.Body).Decode(&got)
			require.NoError(t, err)
			assert.Equal(t, tt.wantErr, got["error"])
		})
	}
}
