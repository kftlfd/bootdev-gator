-- name: ResetDB :exec
TRUNCATE TABLE users, feeds, feed_follows CASCADE;
