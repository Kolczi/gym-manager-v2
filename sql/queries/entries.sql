-- name: ListEntriesByClient :many
SELECT e.id, e.client_id, e.recorded_by, e.method, e.created_at,
       COALESCE(u.name, '') AS recorded_by_name
FROM entries e
LEFT JOIN users u ON u.id = e.recorded_by
WHERE e.client_id = ?1
ORDER BY e.created_at DESC
LIMIT ?2;

-- name: ListEntriesToday :many
SELECT e.id, e.client_id, e.method, e.created_at,
       c.name AS client_name, c.surname AS client_surname
FROM entries e
JOIN clients c ON c.id = e.client_id
WHERE e.created_at >= date('now')
ORDER BY e.created_at DESC;

-- name: CreateEntry :one
INSERT INTO entries (client_id, recorded_by, method)
VALUES (?1, ?2, ?3)
RETURNING *;

-- name: CountEntriesToday :one
SELECT COUNT(*) FROM entries WHERE created_at >= date('now');

-- name: CountEntriesByClient :one
SELECT COUNT(*) FROM entries WHERE client_id = ?1;

-- name: GetLastEntryByClient :one
SELECT id, created_at FROM entries
WHERE client_id = ?1
ORDER BY created_at DESC
LIMIT 1;
