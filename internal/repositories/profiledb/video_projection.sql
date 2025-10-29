-- name: UpsertVideoProjection :exec
INSERT INTO profile.videos_projection (
    video_id,
    title,
    description,
    duration_micros,
    thumbnail_url,
    hls_master_playlist,
    status,
    visibility_status,
    published_at,
    version,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, COALESCE($11, now())
)
ON CONFLICT (video_id) DO UPDATE
SET title               = $2,
    description         = $3,
    duration_micros     = $4,
    thumbnail_url       = $5,
    hls_master_playlist = $6,
    status              = $7,
    visibility_status   = $8,
    published_at        = $9,
    version             = $10,
    updated_at          = COALESCE($11, now());

-- name: GetVideoProjection :one
SELECT
    video_id,
    title,
    description,
    duration_micros,
    thumbnail_url,
    hls_master_playlist,
    status,
    visibility_status,
    published_at,
    version,
    updated_at
FROM profile.videos_projection
WHERE video_id = $1;

-- name: ListVideoProjections :many
SELECT
    video_id,
    title,
    description,
    duration_micros,
    thumbnail_url,
    hls_master_playlist,
    status,
    visibility_status,
    published_at,
    version,
    updated_at
FROM profile.videos_projection
WHERE video_id = ANY($1::uuid[]);
