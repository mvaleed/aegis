package grpc

import (
	"context"

	userv1 "github.com/mvaleed/aegis/api/proto/user/v1"
	"github.com/mvaleed/aegis/internal/domain"
	"github.com/mvaleed/aegis/internal/service"
	"google.golang.org/protobuf/types/known/emptypb"
)

type rbacHandler struct {
	userv1.UnimplementedRBACServiceServer
	rbacService *service.RBACService
}

func NewRBACHandler(rbacService *service.RBACService) userv1.RBACServiceServer {
	return &rbacHandler{rbacService: rbacService}
}

func (h *rbacHandler) CreateRole(ctx context.Context, req *userv1.CreateRoleRequest) (*userv1.CreateRoleResponse, error) {
	if err := requirePermission(ctx, "roles", "create"); err != nil {
		return nil, err
	}

	role, err := h.rbacService.CreateRole(ctx, req.Name, req.Description)
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &userv1.CreateRoleResponse{
		Role: domainRoleToProto(role),
	}, nil
}

func (h *rbacHandler) AssignRole(ctx context.Context, req *userv1.AssignRoleRequest) (*emptypb.Empty, error) {
	if err := requirePermission(ctx, "roles", "assign"); err != nil {
		return nil, err
	}

	if err := h.rbacService.AssignRole(ctx, domain.UUIDFromString(req.UserId), domain.UUIDFromString(req.RoleId)); err != nil {
		return nil, mapDomainError(err)
	}

	return &emptypb.Empty{}, nil
}
