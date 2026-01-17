package grpc

import (
	"context"

	userv1 "github.com/mvaleed/aegis/api/proto/user/v1"
	"github.com/mvaleed/aegis/internal/domain"
	"github.com/mvaleed/aegis/internal/service"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type userHandler struct {
	userv1.UnimplementedUserServiceServer
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) userv1.UserServiceServer {
	return &userHandler{userService: userService}
}

func (h *userHandler) CreateUser(ctx context.Context, req *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	user, err := h.userService.CreateUser(ctx, service.CreateUserInput{
		Email:    req.Email,
		Password: req.Password,
		Username: req.Username,
		FullName: req.FullName,
		Phone:    req.Phone,
		UserType: mapProtoUserType(req.UserType),
	})
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &userv1.CreateUserResponse{
		User: domainUserToProto(user),
	}, nil
}

func (h *userHandler) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	user, err := h.userService.GetUser(ctx, domain.UUIDFromString(req.Id))
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &userv1.GetUserResponse{
		User: domainUserToProto(user),
	}, nil
}

// ... implement remaining methods similarly

// Helper functions for type conversion:

func domainUserToProto(u *domain.User) *userv1.User {
	if u == nil {
		return nil
	}

	roles := make([]*userv1.Role, len(u.Roles))
	for i, r := range u.Roles {
		roles[i] = domainRoleToProto(&r)
	}

	return &userv1.User{
		Id:            u.ID.String(),
		Email:         u.Email,
		Username:      u.Username,
		FullName:      u.FullName,
		Phone:         *u.Phone,
		UserType:      userTypeToProto(u.UserType),
		Status:        userStatusToProto(u.Status),
		EmailVerified: u.EmailVerified,
		PhoneVerified: u.PhoneVerified,
		CreatedAt:     timestamppb.New(u.CreatedAt),
		UpdatedAt:     timestamppb.New(u.UpdatedAt),
		Roles:         roles,
	}
}

func domainRoleToProto(r *domain.Role) *userv1.Role {
	if r == nil {
		return nil
	}

	permissions := make([]*userv1.Permission, len(r.Permissions))
	for i, p := range r.Permissions {
		permissions[i] = domainPermissionToProto(&p)
	}

	return &userv1.Role{
		Id:          r.ID.String(),
		Name:        r.Name,
		Description: r.Description,
		Permissions: permissions,
		CreatedAt:   timestamppb.New(r.CreatedAt),
	}
}

func domainPermissionToProto(p *domain.Permission) *userv1.Permission {
	if p == nil {
		return nil
	}

	return &userv1.Permission{
		Id:          p.ID.String(),
		Resource:    p.Resource,
		Action:      p.Action,
		Description: p.Description,
	}
}

func userTypeToProto(ut domain.UserType) userv1.UserType {
	switch ut {
	case domain.UserTypeAdmin:
		return userv1.UserType_USER_TYPE_ADMIN
	case domain.UserTypeCustomer:
		return userv1.UserType_USER_TYPE_CUSTOMER
	case domain.UserTypePartner:
		return userv1.UserType_USER_TYPE_PARTNER
	default:
		return userv1.UserType_USER_TYPE_UNSPECIFIED
	}
}

func mapProtoUserType(ut userv1.UserType) domain.UserType {
	switch ut {
	case userv1.UserType_USER_TYPE_ADMIN:
		return domain.UserTypeAdmin
	case userv1.UserType_USER_TYPE_CUSTOMER:
		return domain.UserTypeCustomer
	case userv1.UserType_USER_TYPE_PARTNER:
		return domain.UserTypePartner
	default:
		return domain.UserTypeCustomer
	}
}

func userStatusToProto(us domain.UserStatus) userv1.UserStatus {
	switch us {
	case domain.UserStatusPending:
		return userv1.UserStatus_USER_STATUS_PENDING
	case domain.UserStatusActive:
		return userv1.UserStatus_USER_STATUS_ACTIVE
	case domain.UserStatusInactive:
		return userv1.UserStatus_USER_STATUS_INACTIVE
	case domain.UserStatusSuspended:
		return userv1.UserStatus_USER_STATUS_SUSPENDED
	default:
		return userv1.UserStatus_USER_STATUS_UNSPECIFIED
	}
}
