-- name: GetUserByLogin :one
SELECT id, login, name, surname, role, password_hash, created_at
FROM users
WHERE login = ?1;

-- name: GetUser :one
SELECT id, login, name, surname, role, password_hash, created_at
FROM users
WHERE id = ?1;

-- name: ListUsers :many
SELECT id, login, name, surname, role, created_at
FROM users
ORDER BY surname, name;

-- name: CreateUser :one
INSERT INTO users (login, name, surname, role, password_hash)
VALUES (?1, ?2, ?3, ?4, ?5)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET name = ?2, surname = ?3, role = ?4
WHERE id = ?1
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = ?2
WHERE id = ?1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = ?1;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;
