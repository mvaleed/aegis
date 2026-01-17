package grpc

import (
	"context"

	userv1 "github.com/mvaleed/aegis/api/proto/user/v1"
	"github.com/mvaleed/aegis/internal/domain"
	"github.com/mvaleed/aegis/internal/service"
	"google.golang.org/protobuf/types/known/emptypb"
)

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
	result, err := h.authService.Login(ctx, service.LoginInput{
		Email:     "",
		Password:  "",
		IPAddress: "",
		UserAgent: "",
	})
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
		ExpiresIn:    result.ExpiresInSeconds,
		User:         domainUserToProto(user),
	}, nil
}

func (h *authHandler) RefreshToken(ctx context.Context, req *userv1.RefreshTokenRequest) (*userv1.RefreshTokenResponse, error) {
	result, err := h.authService.RefreshToken(ctx, service.RefreshTokenInput{
		RefreshToken: "",
		IPAddress:    "",
		UserAgent:    "",
	})
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &userv1.RefreshTokenResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresInSeconds,
	}, nil
}

func (h *authHandler) Logout(ctx context.Context, req *userv1.LogoutRequest) (*emptypb.Empty, error) {
	if err := h.authService.Logout(ctx, req.RefreshToken); err != nil {
		return nil, mapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

func (h *authHandler) LogoutAll(ctx context.Context, req *userv1.LogoutAllRequest) (*emptypb.Empty, error) {
	if err := h.authService.LogoutAll(ctx, domain.UUIDFromString(req.UserId)); err != nil {
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
		UserId:      claims.UserID.String(),
		Email:       claims.Email,
		UserType:    userTypeToProto(domain.UserType(claims.UserType)),
		Permissions: claims.Permissions,
	}, nil
}
