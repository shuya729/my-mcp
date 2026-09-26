package apihandler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"my-mcp/internal/domain"
	"my-mcp/internal/usecase"

	"github.com/go-playground/validator/v10"
	"github.com/modelcontextprotocol/go-sdk/auth"
)

type UserService interface {
	Get(context.Context, string) (domain.User, error)
	Create(context.Context, string, string) (domain.User, error)
	Update(context.Context, string, string) (domain.User, error)
	Delete(context.Context, string) error
}

type UserHandler struct {
	users    UserService
	validate *validator.Validate
}

type nameRequest struct {
	Name string `json:"name" validate:"required"`
}

type userResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func NewUserHandler(users UserService) *UserHandler {
	return &UserHandler{users: users, validate: validator.New(validator.WithRequiredStructEnabled())}
}

func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	user, err := h.users.Get(r.Context(), auth.TokenInfoFromContext(r.Context()).UserID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeUser(w, http.StatusOK, user)
}

func (h *UserHandler) CreateMe(w http.ResponseWriter, r *http.Request) {
	name, err := h.decodeName(w, r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid name or JSON body")
		return
	}
	user, err := h.users.Create(r.Context(), auth.TokenInfoFromContext(r.Context()).UserID, name)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeUser(w, http.StatusCreated, user)
}

func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	name, err := h.decodeName(w, r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid name or JSON body")
		return
	}
	user, err := h.users.Update(r.Context(), auth.TokenInfoFromContext(r.Context()).UserID, name)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeUser(w, http.StatusOK, user)
}

func (h *UserHandler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	if err := h.users.Delete(r.Context(), auth.TokenInfoFromContext(r.Context()).UserID); err != nil {
		h.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) decodeName(w http.ResponseWriter, r *http.Request) (string, error) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var request nameRequest
	if err := decoder.Decode(&request); err != nil {
		return "", err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return "", errors.New("unexpected content after JSON body")
	}
	if err := h.validate.Struct(request); err != nil {
		return "", err
	}
	return request.Name, nil
}

func (h *UserHandler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, usecase.ErrUserNotFound):
		writeJSONError(w, http.StatusNotFound, "user not found")
	case errors.Is(err, usecase.ErrUserExists):
		writeJSONError(w, http.StatusConflict, "user already exists")
	case errors.Is(err, usecase.ErrInvalidName):
		writeJSONError(w, http.StatusBadRequest, "name is required")
	default:
		slog.Error("user request failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal server error")
	}
}

func writeUser(w http.ResponseWriter, status int, user domain.User) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(userResponse{ID: user.ID, Name: user.Name})
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{Error: message})
}
