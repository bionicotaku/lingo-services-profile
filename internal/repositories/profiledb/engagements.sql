-- name: UpsertEngagement :exec
INSERT INTO profile.engagements (
    user_id,
    video_id,
    engagement_type,
    created_at,
    updated_at,
    deleted_at
) VALUES (
    $1, $2, $3, COALESCE($4, now()), COALESCE($4, now()), NULL
)
ON CONFLICT (user_id, video_id, engagement_type) DO UPDATE
SET deleted_at = NULL,
    updated_at = COALESCE($4, now()),
    created_at = profile.engagements.created_at;

-- name: SoftDeleteEngagement :exec
UPDATE profile.engagements
SET deleted_at = $4,
    updated_at = COALESCE($4, now())
WHERE user_id = $1
  AND video_id = $2
  AND engagement_type = $3;

-- name: GetEngagement :one
SELECT
    user_id,
    video_id,
    engagement_type,
    created_at,
    updated_at,
    deleted_at
FROM profile.engagements
WHERE user_id = $1
  AND video_id = $2
  AND engagement_type = $3;

-- name: ListEngagementsByUser :many
SELECT
    user_id,
    video_id,
    engagement_type,
    created_at,
    updated_at,
    deleted_at
FROM profile.engagements
WHERE user_id = $1
  AND ($2 = '' OR engagement_type = $2)
  AND (deleted_at IS NULL OR $3 = false)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;
