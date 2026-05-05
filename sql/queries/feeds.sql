-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: GetAllFeeds :many
SELECT
    feeds.name as feed_name,
    feeds.url as url,
    users.name as user_name
FROM feeds JOIN users on users.id = feeds.user_id;

-- name: ResetFeeds :exec
TRUNCATE TABLE feeds CASCADE;