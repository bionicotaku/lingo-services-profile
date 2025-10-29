-- name: UpsertVideoStats :exec
INSERT INTO profile.video_stats (
    video_id,
    like_count,
    bookmark_count,
    unique_watchers,
    total_watch_seconds,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, COALESCE($6, now())
)
ON CONFLICT (video_id) DO UPDATE
SET like_count          = profile.video_stats.like_count + $2,
    bookmark_count      = profile.video_stats.bookmark_count + $3,
    unique_watchers     = profile.video_stats.unique_watchers + $4,
    total_watch_seconds = profile.video_stats.total_watch_seconds + $5,
    updated_at          = COALESCE($6, now());

-- name: SetVideoStats :exec
UPDATE profile.video_stats
SET like_count          = $2,
    bookmark_count      = $3,
    unique_watchers     = $4,
    total_watch_seconds = $5,
    updated_at          = COALESCE($6, now())
WHERE video_id = $1;

-- name: GetVideoStats :one
SELECT
    video_id,
    like_count,
    bookmark_count,
    unique_watchers,
    total_watch_seconds,
    updated_at
FROM profile.video_stats
WHERE video_id = $1;

-- name: ListVideoStats :many
SELECT
    video_id,
    like_count,
    bookmark_count,
    unique_watchers,
    total_watch_seconds,
    updated_at
FROM profile.video_stats
WHERE video_id = ANY($1::uuid[]);
