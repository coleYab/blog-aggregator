-- name: CreateFeedFollows :one
INSERT INTO feed_follows (user_id, feed_id)
VALUES ($1, $2) RETURNING *;

-- name: GetFeedFollowsForUser :many
SELECT feeds.*
From
    feeds INNER JOIN
    feed_follows on feed_follows.feed_id = feeds.id
WHERE feed_follows.user_id = $1;


-- name: GetFeedByFeedIdAndUserId :one
SELECT *
FROM feed_follows
WHERE user_id=$1 and feed_id=$2
LIMIT 1;

-- name: DeleteFollowFeed :exec
DELETE FROM feed_follows
WHERE user_id=$1 and feed_id=$2;
