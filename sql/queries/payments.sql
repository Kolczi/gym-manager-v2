-- name: ListPaymentsByMembership :many
SELECT id, membership_id, due_date, paid_at, amount
FROM payments
WHERE membership_id = ?1
ORDER BY due_date DESC;

-- name: ListPaymentsByClient :many
SELECT p.id, p.membership_id, p.due_date, p.paid_at, p.amount,
       mt.name AS type_name
FROM payments p
JOIN memberships m ON m.id = p.membership_id
JOIN membership_types mt ON mt.id = m.type_id
WHERE m.client_id = ?1
ORDER BY p.due_date DESC;

-- name: ListOverduePayments :many
SELECT p.id, p.membership_id, p.due_date, p.amount,
       c.id AS client_id, c.name AS client_name, c.surname AS client_surname,
       mt.name AS type_name
FROM payments p
JOIN memberships m ON m.id = p.membership_id
JOIN clients c ON c.id = m.client_id
JOIN membership_types mt ON mt.id = m.type_id
WHERE p.paid_at IS NULL AND p.due_date < date('now')
ORDER BY p.due_date;

-- name: CreatePayment :one
INSERT INTO payments (membership_id, due_date, amount)
VALUES (?1, ?2, ?3)
RETURNING *;

-- name: MarkPaymentPaid :exec
UPDATE payments SET paid_at = datetime('now') WHERE id = ?1;

-- name: CountOverduePayments :one
SELECT COUNT(*) FROM payments WHERE paid_at IS NULL AND due_date < date('now');

-- name: CountUnpaidPayments :one
SELECT COUNT(*) FROM payments WHERE paid_at IS NULL;
