-- name: ListMembershipTypes :many
SELECT id, name, description, duration_value, duration_unit, is_contract, price, is_active, max_freeze_days, created_at
FROM membership_types
ORDER BY is_active DESC, name;

-- name: ListActiveMembershipTypes :many
SELECT id, name, description, duration_value, duration_unit, is_contract, price, max_freeze_days
FROM membership_types
WHERE is_active = 1
ORDER BY name;

-- name: GetMembershipType :one
SELECT * FROM membership_types WHERE id = ?1;

-- name: CreateMembershipType :one
INSERT INTO membership_types (name, description, duration_value, duration_unit, is_contract, price, is_active, max_freeze_days)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8)
RETURNING *;

-- name: UpdateMembershipType :one
UPDATE membership_types
SET name = ?2, description = ?3, duration_value = ?4, duration_unit = ?5, is_contract = ?6, price = ?7, is_active = ?8, max_freeze_days = ?9
WHERE id = ?1
RETURNING *;

-- name: DeleteMembershipType :exec
DELETE FROM membership_types WHERE id = ?1;

-- name: CountMembershipTypes :one
SELECT COUNT(*) FROM membership_types;
