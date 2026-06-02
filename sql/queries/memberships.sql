-- name: ListMembershipsByClient :many
SELECT m.id, m.client_id, m.type_id, m.starts_at, m.ends_at, m.is_active,
       m.frozen_at, m.frozen_until, m.total_frozen_days, m.created_at,
       mt.name AS type_name, mt.price AS type_price, mt.max_freeze_days
FROM memberships m
JOIN membership_types mt ON mt.id = m.type_id
WHERE m.client_id = ?1
ORDER BY m.starts_at DESC;

-- name: GetMembership :one
SELECT m.*, mt.name AS type_name, mt.price AS type_price, mt.max_freeze_days
FROM memberships m
JOIN membership_types mt ON mt.id = m.type_id
WHERE m.id = ?1;

-- name: GetActiveMembership :one
SELECT m.id, m.client_id, m.type_id, m.starts_at, m.ends_at, m.is_active,
       m.frozen_at, m.frozen_until, m.total_frozen_days,
       mt.name AS type_name
FROM memberships m
JOIN membership_types mt ON mt.id = m.type_id
WHERE m.client_id = ?1 AND m.is_active = 1 AND m.ends_at >= date('now')
LIMIT 1;

-- name: CreateMembership :one
INSERT INTO memberships (client_id, type_id, starts_at, ends_at, is_active)
VALUES (?1, ?2, ?3, ?4, ?5)
RETURNING *;

-- name: DeactivateMembership :exec
UPDATE memberships SET is_active = 0, frozen_at = NULL WHERE id = ?1;

-- name: CountActiveMemberships :one
SELECT COUNT(*) FROM memberships WHERE is_active = 1 AND ends_at >= date('now');

-- name: CountExpiredMemberships :one
SELECT COUNT(*) FROM memberships WHERE is_active = 1 AND ends_at < date('now');

-- name: HasOverlappingMembership :one
SELECT COUNT(*) > 0 AS has_overlap
FROM memberships
WHERE client_id = ?1
  AND is_active = 1
  AND starts_at <= @new_ends_at
  AND ends_at >= @new_starts_at;

-- name: FreezeMembership :exec
UPDATE memberships
SET frozen_at = @freeze_from,
    frozen_until = @freeze_until
WHERE id = ?1;

-- name: UnfreezeMembership :exec
UPDATE memberships
SET total_frozen_days = total_frozen_days + CAST(julianday(COALESCE(frozen_until, date('now'))) - julianday(frozen_at) AS INTEGER),
    ends_at = date(ends_at, '+' || CAST(julianday(COALESCE(frozen_until, date('now'))) - julianday(frozen_at) AS INTEGER) || ' days'),
    frozen_at = NULL,
    frozen_until = NULL
WHERE id = ?1 AND frozen_at IS NOT NULL;
