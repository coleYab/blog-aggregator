-- name: GetFeeds :many
SELECT * FROM feeds;

-- name: SelectFeedsWithCreator :many
SELECT feeds.name, feeds.url, users.name AS user_name
FROM feeds
JOIN users ON feeds.user_id = users.id;

-- name: GetFeedByURL :one
select * from feeds
where url = $1
limit 1;

-- name: CreateFeed :one
INSERT INTO feeds (name, url, user_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: MarkFeedFetched :one
UPDATE feeds SET
    last_fetched_at=CURRENT_TIMESTAMP,
    updated_at=CURRENT_TIMESTAMP
WHERE id=$1
RETURNING *;

-- name: GetNextFeedToFetch :one
SELECT * FROM feeds
ORDER BY last_fetched_at DESC NULLS FIRST
LIMIT 1;
