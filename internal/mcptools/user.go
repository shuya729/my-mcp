package mcptools

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"my-mcp/internal/domain"
	"my-mcp/internal/usecase"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ReadScope  = "user:read"
	WriteScope = "user:write"
)

type UserService interface {
	Get(context.Context, string) (domain.User, error)
	Create(context.Context, string, string) (domain.User, error)
	Update(context.Context, string, string) (domain.User, error)
	Delete(context.Context, string) error
}

type UserTools struct {
	Users UserService
}

type NameInput struct {
	Name string `json:"name"`
}

type UserOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (t UserTools) GetUser(ctx context.Context, request *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, UserOutput, error) {
	authID, err := subjectFor(request, ReadScope)
	if err != nil {
		return nil, UserOutput{}, err
	}
	user, err := t.Users.Get(ctx, authID)
	if err != nil {
		return nil, UserOutput{}, userError(err)
	}
	return nil, UserOutput{ID: user.ID, Name: user.Name}, nil
}

func (t UserTools) CreateUser(ctx context.Context, request *mcp.CallToolRequest, input NameInput) (*mcp.CallToolResult, UserOutput, error) {
	authID, err := subjectFor(request, WriteScope)
	if err != nil {
		return nil, UserOutput{}, err
	}
	user, err := t.Users.Create(ctx, authID, input.Name)
	if err != nil {
		return nil, UserOutput{}, userError(err)
	}
	return nil, UserOutput{ID: user.ID, Name: user.Name}, nil
}

func (t UserTools) UpdateUser(ctx context.Context, request *mcp.CallToolRequest, input NameInput) (*mcp.CallToolResult, UserOutput, error) {
	authID, err := subjectFor(request, WriteScope)
	if err != nil {
		return nil, UserOutput{}, err
	}
	user, err := t.Users.Update(ctx, authID, input.Name)
	if err != nil {
		return nil, UserOutput{}, userError(err)
	}
	return nil, UserOutput{ID: user.ID, Name: user.Name}, nil
}

func (t UserTools) DeleteUser(ctx context.Context, request *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, struct{}, error) {
	authID, err := subjectFor(request, WriteScope)
	if err != nil {
		return nil, struct{}{}, err
	}
	if err := t.Users.Delete(ctx, authID); err != nil {
		return nil, struct{}{}, userError(err)
	}
	return nil, struct{}{}, nil
}

func subjectFor(request *mcp.CallToolRequest, scope string) (string, error) {
	if request == nil || request.Extra == nil || request.Extra.TokenInfo == nil || request.Extra.TokenInfo.UserID == "" {
		return "", errors.New("authenticated identity is unavailable")
	}
	if !slices.Contains(request.Extra.TokenInfo.Scopes, scope) {
		return "", errors.New("required scope: " + scope)
	}
	return request.Extra.TokenInfo.UserID, nil
}

func userError(err error) error {
	switch {
	case errors.Is(err, usecase.ErrUserNotFound), errors.Is(err, usecase.ErrUserExists), errors.Is(err, usecase.ErrInvalidName):
		return err
	default:
		slog.Error("MCP user tool failed", "error", err)
		return errors.New("internal server error")
	}
}
