-- name: GetProfileUser :one
SELECT
    user_id,
    display_name,
    avatar_url,
    profile_version,
    preferences_json,
    created_at,
    updated_at
FROM profile.users
WHERE user_id = $1;

-- name: UpsertProfileUser :one
INSERT INTO profile.users (
    user_id,
    display_name,
    avatar_url,
    profile_version,
    preferences_json
) VALUES (
    $1, $2, $3, $4, $5
)
ON CONFLICT (user_id) DO UPDATE
SET display_name = EXCLUDED.display_name,
    avatar_url = EXCLUDED.avatar_url,
    profile_version = EXCLUDED.profile_version,
    preferences_json = EXCLUDED.preferences_json,
    updated_at = now()
RETURNING
    user_id,
    display_name,
    avatar_url,
    profile_version,
    preferences_json,
    created_at,
    updated_at;
