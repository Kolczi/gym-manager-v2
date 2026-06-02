-- name: CountActiveClients :one
SELECT COUNT(*) FROM clients;

-- name: CountExpiringMemberships :one
SELECT COUNT(*) FROM memberships WHERE is_active = 1 AND ends_at BETWEEN date('now') AND date('now', '+7 days');

-- name: ListExpiringMemberships :many
SELECT m.id, m.ends_at, c.id AS client_id, c.name AS client_name, c.surname AS client_surname,
       mt.name AS type_name
FROM memberships m
JOIN clients c ON c.id = m.client_id
JOIN membership_types mt ON mt.id = m.type_id
WHERE m.is_active = 1 AND m.ends_at BETWEEN date('now') AND date('now', '+7 days')
ORDER BY m.ends_at;

-- name: ListRecentEntries :many
SELECT e.id, e.method, e.created_at,
       c.id AS client_id, c.name AS client_name, c.surname AS client_surname
FROM entries e
JOIN clients c ON c.id = e.client_id
WHERE e.created_at >= date('now')
ORDER BY e.created_at DESC
LIMIT 10;
