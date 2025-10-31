package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/BuzzLyutic/task-manager-api/internal/model"
	"github.com/BuzzLyutic/task-manager-api/internal/repo"
	"github.com/BuzzLyutic/task-manager-api/internal/service"
	"github.com/BuzzLyutic/task-manager-api/pkg/respond"
)

type TaskHandler struct {
	service *service.TaskService
	logger  *zap.Logger
}

func NewTaskHandler(srv *service.TaskService, logger *zap.Logger) *TaskHandler {
	return &TaskHandler{
		service: srv,
		logger:  logger,
	}
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {

	if r.ContentLength == 0 {
		respond.Error(w, r, http.StatusBadRequest, "empty request body")
		return
	}

	var req model.Task
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("failed to decode json", zap.Error(err))
		respond.Error(w, r, http.StatusBadRequest, fmt.Sprintf("invalid json: %v", err))
		return
	}

	idempKey := r.Header.Get("Idempotency-Key")
	task, err := h.service.Create(r.Context(), req, idempKey)
	if err != nil {
		h.handleErrors(w, r, err)
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/api/tasks/%d", task.ID))
	respond.JSON(w, r, http.StatusCreated, task)
}

func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	task, err := h.service.Get(r.Context(), id)
	if err != nil {
		h.handleErrors(w, r, err)
		return
	}
	respond.JSON(w, r, http.StatusOK, task)
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	var filter model.TaskFilter
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = &status
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	tasks, err := h.service.List(r.Context(), filter, limit)
	if err != nil {
		h.handleErrors(w, r, err)
	}
	respond.JSON(w, r, http.StatusOK, tasks)
}

func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	var req model.Task
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, r, http.StatusBadRequest, "invalid json")
		return
	}
	req.ID = id

	task, err := h.service.Update(r.Context(), req)
	if err != nil {
		h.handleErrors(w, r, err)
		return
	}

	respond.JSON(w, r, http.StatusOK, task)
}

func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	if err := h.service.Delete(r.Context(), id); err != nil {
		h.handleErrors(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TaskHandler) handleErrors(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, repo.ErrorNotFound):
		respond.Error(w, r, http.StatusNotFound, "not found")
	case errors.Is(err, repo.ErrorConflict):
		respond.Error(w, r, http.StatusConflict, "conflict")
	case errors.Is(err, service.ErrValidation):
		respond.Error(w, r, http.StatusBadRequest, "validation error")
	default:
		h.logger.Error("internal error", zap.Error(err))
		respond.Error(w, r, http.StatusInternalServerError, "internal error")
	}
}
