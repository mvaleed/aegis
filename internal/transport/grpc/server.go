// Package grpc provides gRPC transport layer for the user service.
//
// This package requires protobuf code generation before use:
//
//	buf generate
//
// The generated code will be placed in api/proto/user/v1/
package grpc

import (
	"context"
	"log/slog"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/mvaleed/aegis/internal/auth"
	"github.com/mvaleed/aegis/internal/service"
)

// Server wraps the gRPC server with dependencies
type Server struct {
	grpcServer  *grpc.Server
	userService *service.UserService
	authService *service.AuthService
	rbacService *service.RBACService
	jwtManager  *auth.JWTManager
	logger      *slog.Logger
}

// NewServer creates a new gRPC server with all handlers registered
func NewServer(
	userService *service.UserService,
	authService *service.AuthService,
	rbacService *service.RBACService,
	jwtManager *auth.JWTManager,
	logger *slog.Logger,
) *Server {
	s := &Server{
		userService: userService,
		authService: authService,
		rbacService: rbacService,
		jwtManager:  jwtManager,
		logger:      logger,
	}

	// Create gRPC server with interceptors
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			s.loggingInterceptor,
			s.recoveryInterceptor,
			s.authInterceptor,
		),
	)

	// Register service handlers
	// NOTE: After running `buf generate`, uncomment these lines and import
	// the generated package:
	//   userv1 "user-service/api/proto/user/v1"
	//
	// userv1.RegisterUserServiceServer(grpcServer, NewUserHandler(s))
	// userv1.RegisterAuthServiceServer(grpcServer, NewAuthHandler(s))
	// userv1.RegisterRBACServiceServer(grpcServer, NewRBACHandler(s))

	s.grpcServer = grpcServer
	return s
}

// Serve starts the gRPC server on the given listener
func (s *Server) Serve(listener net.Listener) error {
	return s.grpcServer.Serve(listener)
}

// GracefulStop gracefully stops the gRPC server
func (s *Server) GracefulStop() {
	s.grpcServer.GracefulStop()
}

// loggingInterceptor logs all incoming requests
func (s *Server) loggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	s.logger.Info("gRPC request",
		"method", info.FullMethod,
	)

	resp, err := handler(ctx, req)
	if err != nil {
		s.logger.Error("gRPC request failed",
			"method", info.FullMethod,
			"error", err,
		)
	}

	return resp, err
}

// recoveryInterceptor recovers from panics
func (s *Server) recoveryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("gRPC panic recovered",
				"method", info.FullMethod,
				"panic", r,
			)
			err = status.Error(codes.Internal, "internal server error")
		}
	}()

	return handler(ctx, req)
}

// authInterceptor validates JWT tokens for protected endpoints
func (s *Server) authInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	// Skip auth for public endpoints
	if isPublicMethod(info.FullMethod) {
		return handler(ctx, req)
	}

	// Extract token from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	tokens := md.Get("authorization")
	if len(tokens) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization token")
	}

	token := tokens[0]
	// Remove "Bearer " prefix if present
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	// Validate token
	claims, err := s.jwtManager.ValidateAccessToken(token)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	// Add claims to context
	ctx = context.WithValue(ctx, claimsKey{}, claims)

	return handler(ctx, req)
}

// claimsKey is the context key for JWT claims
type claimsKey struct{}

// ClaimsFromContext extracts JWT claims from the context
func ClaimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	claims, ok := ctx.Value(claimsKey{}).(*auth.Claims)
	return claims, ok
}

// isPublicMethod returns true if the method doesn't require authentication
func isPublicMethod(method string) bool {
	publicMethods := map[string]bool{
		"/user.v1.AuthService/Login":        true,
		"/user.v1.AuthService/RefreshToken": true,
		"/user.v1.UserService/CreateUser":   true,
	}
	return publicMethods[method]
}

// requirePermission checks if the current user has the required permission
func requirePermission(ctx context.Context, resource, action string) error {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "not authenticated")
	}

	requiredPerm := resource + ":" + action
	for _, perm := range claims.Permissions {
		if perm == requiredPerm || perm == "*:*" || perm == resource+":*" {
			return nil
		}
	}

	return status.Error(codes.PermissionDenied, "permission denied")
}
