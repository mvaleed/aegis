package grpc

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mvaleed/aegis/internal/domain"
	"github.com/mvaleed/aegis/internal/service"
)

// NOTE: This file contains handler implementations that work with the generated
// protobuf types. After running `buf generate`, import the generated package:
//
//   userv1 "user-service/api/proto/user/v1"
//
// Then uncomment and adjust the handlers below.

// mapDomainError converts domain errors to gRPC status errors
func mapDomainError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidCredential):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrInvalidStatus):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrConcurrentModification):
		return status.Error(codes.Aborted, err.Error())
	case errors.Is(err, domain.ErrTokenExpired):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrTokenRevoked):
		return status.Error(codes.Unauthenticated, err.Error())
	}

	var validationErr *domain.ValidationError
	if errors.As(err, &validationErr) {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	var validationErrs domain.ValidationErrors
	if errors.As(err, &validationErrs) {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	return status.Error(codes.Internal, "internal server error")
}

/*
================================================================================
USER SERVICE HANDLER IMPLEMENTATION
================================================================================

After running `buf generate`, create user_handler.go with:

package grpc

import (
    "context"

    userv1 "user-service/api/proto/user/v1"
    "user-service/internal/domain"
    "user-service/internal/service"
    "google.golang.org/protobuf/types/known/emptypb"
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
    user, err := h.userService.GetUser(ctx, req.Id)
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
        Phone:         u.Phone,
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

================================================================================
AUTH SERVICE HANDLER IMPLEMENTATION
================================================================================

type authHandler struct {
    userv1.UnimplementedAuthServiceServer
    authService *service.AuthService
    userService *service.UserService
}

func NewAuthHandler(authService *service.AuthService, userService *service.UserService) userv1.AuthServiceServer {
    return &authHandler{
        authService: authService,
        userService: userService,
    }
}

func (h *authHandler) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginResponse, error) {
    result, err := h.authService.Login(ctx, req.Email, req.Password, req.IpAddress, req.UserAgent)
    if err != nil {
        return nil, mapDomainError(err)
    }

    user, err := h.userService.GetUserByEmail(ctx, req.Email)
    if err != nil {
        return nil, mapDomainError(err)
    }

    return &userv1.LoginResponse{
        AccessToken:  result.AccessToken,
        RefreshToken: result.RefreshToken,
        ExpiresIn:    result.ExpiresIn,
        User:         domainUserToProto(user),
    }, nil
}

func (h *authHandler) RefreshToken(ctx context.Context, req *userv1.RefreshTokenRequest) (*userv1.RefreshTokenResponse, error) {
    result, err := h.authService.RefreshToken(ctx, req.RefreshToken, req.IpAddress, req.UserAgent)
    if err != nil {
        return nil, mapDomainError(err)
    }

    return &userv1.RefreshTokenResponse{
        AccessToken:  result.AccessToken,
        RefreshToken: result.RefreshToken,
        ExpiresIn:    result.ExpiresIn,
    }, nil
}

func (h *authHandler) Logout(ctx context.Context, req *userv1.LogoutRequest) (*emptypb.Empty, error) {
    if err := h.authService.Logout(ctx, req.RefreshToken); err != nil {
        return nil, mapDomainError(err)
    }
    return &emptypb.Empty{}, nil
}

func (h *authHandler) LogoutAll(ctx context.Context, req *userv1.LogoutAllRequest) (*emptypb.Empty, error) {
    if err := h.authService.LogoutAll(ctx, req.UserId); err != nil {
        return nil, mapDomainError(err)
    }
    return &emptypb.Empty{}, nil
}

func (h *authHandler) ValidateToken(ctx context.Context, req *userv1.ValidateTokenRequest) (*userv1.ValidateTokenResponse, error) {
    claims, err := h.authService.ValidateToken(ctx, req.AccessToken)
    if err != nil {
        return &userv1.ValidateTokenResponse{Valid: false}, nil
    }

    return &userv1.ValidateTokenResponse{
        Valid:       true,
        UserId:      claims.UserID,
        Email:       claims.Email,
        UserType:    userTypeToProto(domain.UserType(claims.UserType)),
        Permissions: claims.Permissions,
    }, nil
}

================================================================================
RBAC SERVICE HANDLER IMPLEMENTATION
================================================================================

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

    if err := h.rbacService.AssignRole(ctx, req.UserId, req.RoleId); err != nil {
        return nil, mapDomainError(err)
    }

    return &emptypb.Empty{}, nil
}

// ... implement remaining RBAC methods similarly

*/

// Placeholder types for compilation (remove after buf generate)
var (
	_ = emptypb.Empty{}
	_ = timestamppb.Timestamp{}
	_ = domain.User{}
	_ = service.UserService{}
)
