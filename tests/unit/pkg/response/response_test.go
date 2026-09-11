package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/racenak/Realtime-Order-Inventory-System/pkg/response"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSON_Success(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	response.JSON(w, r, http.StatusOK, map[string]string{"id": "123"})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var resp response.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestJSON_Created(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", nil)

	response.JSON(w, r, http.StatusCreated, map[string]string{"id": "123"})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestJSON_Error(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	response.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Resource not found")

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp response.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "NOT_FOUND", resp.Error.Code)
	assert.Equal(t, "Resource not found", resp.Error.Message)
}

func TestJSONWithPagination(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	data := []map[string]string{{"id": "1"}, {"id": "2"}}
	response.JSONWithPagination(w, r, http.StatusOK, data, 10, 5, 0)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response.PaginatedResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, 10, resp.Meta.Total)
	assert.Equal(t, 5, resp.Meta.Limit)
	assert.Equal(t, 0, resp.Meta.Offset)
	assert.Equal(t, 2, resp.Meta.TotalPages) // ceil(10/5)
}

func TestJSONWithPagination_ZeroLimit(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	response.JSONWithPagination(w, r, http.StatusOK, nil, 10, 0, 0)

	var resp response.PaginatedResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	// With zero limit, totalPages calculation is undefined (division by zero)
	// The function should handle this gracefully
	assert.Equal(t, 10, resp.Meta.Total)
	assert.Equal(t, 0, resp.Meta.Limit)
}
