-- name: UpsertWatchLog :exec
INSERT INTO profile.watch_logs (
    user_id,
    video_id,
    position_seconds,
    progress_ratio,
    total_watch_seconds,
    first_watched_at,
    last_watched_at,
    expires_at,
    redacted_at
) VALUES (
    $1, $2, $3, $4, $5, COALESCE($6, now()), COALESCE($7, now()), $8, $9
)
ON CONFLICT (user_id, video_id) DO UPDATE
SET position_seconds    = $3,
    progress_ratio      = $4,
    total_watch_seconds = profile.watch_logs.total_watch_seconds + $10,
    last_watched_at     = COALESCE($7, now()),
    expires_at          = $8,
    redacted_at         = $9,
    updated_at          = now();

-- name: GetWatchLog :one
SELECT
    user_id,
    video_id,
    position_seconds,
    progress_ratio,
    total_watch_seconds,
    first_watched_at,
    last_watched_at,
    expires_at,
    redacted_at,
    created_at,
    updated_at
FROM profile.watch_logs
WHERE user_id = $1
  AND video_id = $2;

-- name: ListWatchLogsByUser :many
SELECT
    user_id,
    video_id,
    position_seconds,
    progress_ratio,
    total_watch_seconds,
    first_watched_at,
    last_watched_at,
    expires_at,
    redacted_at,
    created_at,
    updated_at
FROM profile.watch_logs
WHERE user_id = $1
  AND (redacted_at IS NULL OR $2::boolean = false)
ORDER BY last_watched_at DESC, video_id DESC
LIMIT $3 OFFSET $4;
