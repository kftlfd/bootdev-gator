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

-- name: GetFeedByUrl :one
SELECT * FROM feeds WHERE url = $1;

-- name: ResetFeeds :exec
TRUNCATE TABLE feeds CASCADE;
TRUNCATE TABLE feed_follows;

-- name: CreateFeedFollow :one
WITH follow as (
    INSERT INTO feed_follows (id, created_at, updated_at, user_id, feed_id)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING *
)

SELECT
    follow.*,
    users.name as user_name,
    feeds.name as feed_name
FROM
    follow
    INNER JOIN users on follow.user_id = users.id
    INNER JOIN feeds on follow.feed_id = feeds.id
;

-- name: GetFeedFollowsForUser :many
SELECT feeds.*
FROM feed_follows INNER JOIN feeds on feed_follows.feed_id = feeds.id
WHERE feed_follows.user_id = $1;

-- name: DeleteFollow :exec
DELETE FROM feed_follows
WHERE user_id = $1 AND feed_id = $2;
