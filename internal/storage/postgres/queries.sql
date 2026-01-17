-- name: CreateUser :one
INSERT INTO users (
    id, email, password_hash, phone, username, full_name,
    user_type, status, email_verified, phone_verified,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByIDIncludeDeleted :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1 AND deleted_at IS NULL;

-- name: UpdateUser :one
UPDATE users SET
    email = $2,
    password_hash = $3,
    phone = $4,
    username = $5,
    full_name = $6,
    user_type = $7,
    status = $8,
    email_verified = $9,
    phone_verified = $10,
    updated_at = $11,
    version = version + 1
WHERE id = $1 AND version = $12 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteUser :exec
UPDATE users SET
    deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListUsers :many
SELECT * FROM users
WHERE deleted_at IS NULL
    AND ($1::user_status IS NULL OR status = $1)
    AND ($2::user_type IS NULL OR user_type = $2)
    AND ($3::text = '' OR 
         LOWER(email) LIKE LOWER('%' || $3 || '%') OR
         LOWER(username) LIKE LOWER('%' || $3 || '%') OR
         LOWER(full_name) LIKE LOWER('%' || $3 || '%'))
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountUsers :one
SELECT COUNT(*) FROM users
WHERE deleted_at IS NULL
    AND ($1::user_status IS NULL OR status = $1)
    AND ($2::user_type IS NULL OR user_type = $2)
    AND ($3::text = '' OR 
         LOWER(email) LIKE LOWER('%' || $3 || '%') OR
         LOWER(username) LIKE LOWER('%' || $3 || '%') OR
         LOWER(full_name) LIKE LOWER('%' || $3 || '%'));

-- name: CreateRole :one
INSERT INTO roles (id, name, description, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetRoleByID :one
SELECT * FROM roles WHERE id = $1;

-- name: GetRoleByName :one
SELECT * FROM roles WHERE name = $1;

-- name: UpdateRole :one
UPDATE roles SET
    name = $2,
    description = $3,
    updated_at = $4
WHERE id = $1
RETURNING *;

-- name: DeleteRole :exec
DELETE FROM roles WHERE id = $1;

-- name: ListRoles :many
SELECT * FROM roles ORDER BY name;

-- name: GetUserRoles :many
SELECT r.* FROM roles r
JOIN user_roles ur ON r.id = ur.role_id
WHERE ur.user_id = $1
ORDER BY r.name;

-- name: AssignRoleToUser :exec
INSERT INTO user_roles (user_id, role_id)
VALUES ($1, $2)
ON CONFLICT (user_id, role_id) DO NOTHING;

-- name: RemoveRoleFromUser :exec
DELETE FROM user_roles
WHERE user_id = $1 AND role_id = $2;

-- name: CountUsersWithRole :one
SELECT COUNT(*) FROM user_roles WHERE role_id = $1;

-- name: CreatePermission :one
INSERT INTO permissions (id, resource, action, description, created_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetPermissionByID :one
SELECT * FROM permissions WHERE id = $1;

-- name: GetPermissionByResourceAction :one
SELECT * FROM permissions WHERE resource = $1 AND action = $2;

-- name: ListPermissions :many
SELECT * FROM permissions ORDER BY resource, action;

-- name: DeletePermission :exec
DELETE FROM permissions WHERE id = $1;

-- name: GetRolePermissions :many
SELECT p.* FROM permissions p
JOIN role_permissions rp ON p.id = rp.permission_id
WHERE rp.role_id = $1
ORDER BY p.resource, p.action;

-- name: AssignPermissionToRole :exec
INSERT INTO role_permissions (role_id, permission_id)
VALUES ($1, $2)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- name: RemovePermissionFromRole :exec
DELETE FROM role_permissions
WHERE role_id = $1 AND permission_id = $2;

-- name: CountRolesWithPermission :one
SELECT COUNT(*) FROM role_permissions WHERE permission_id = $1;

-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (
    id, user_id, token_hash, expires_at, created_at,
    ip_address, user_agent
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_tokens WHERE token_hash = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked_at = NOW()
WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeAllUserRefreshTokens :exec
UPDATE refresh_tokens SET revoked_at = NOW()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: MarkTokenReplaced :exec
UPDATE refresh_tokens SET
    revoked_at = NOW(),
    replaced_by_id = $2
WHERE id = $1;

-- name: DeleteExpiredTokens :execrows
DELETE FROM refresh_tokens
WHERE expires_at < NOW() - INTERVAL '7 days';
