package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/bionicotaku/lingo-services-profile/internal/models/vo"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories"

	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

var (
	// ErrProfileNotFound 表示档案不存在。
	ErrProfileNotFound = errors.New("profile not found")
	// ErrProfileVersionConflict 表示乐观锁版本冲突。
	ErrProfileVersionConflict = errors.New("profile version conflict")
)

// ProfileService 负责档案与偏好相关的业务逻辑。
type ProfileService struct {
	repo      *repositories.ProfileUsersRepository
	txManager txmanager.Manager
	log       *log.Helper
}

// NewProfileService 构造 ProfileService。
func NewProfileService(repo *repositories.ProfileUsersRepository, tx txmanager.Manager, logger log.Logger) *ProfileService {
	return &ProfileService{
		repo:      repo,
		txManager: tx,
		log:       log.NewHelper(logger),
	}
}

// GetProfile 返回指定用户的档案信息。
func (s *ProfileService) GetProfile(ctx context.Context, userID uuid.UUID) (*vo.Profile, error) {
	record, err := s.repo.Get(ctx, nil, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrProfileUserNotFound) {
			return nil, ErrProfileNotFound
		}
		return nil, fmt.Errorf("get profile: %w", err)
	}
	prefs := toPreferencesVO(record.PreferencesJSON)
	return vo.NewProfileFromPO(record, prefs), nil
}

// UpdateProfileInput 描述档案基础信息更新参数。
type UpdateProfileInput struct {
	UserID           uuid.UUID
	DisplayName      *string
	AvatarURL        *string
	ExpectedVersion  *int64
	PreferencesPatch *vo.Preferences
}

// UpdateProfile 更新档案基础信息，如果不存在则创建。
func (s *ProfileService) UpdateProfile(ctx context.Context, input UpdateProfileInput) (*vo.Profile, error) {
	if input.DisplayName == nil && input.AvatarURL == nil && input.PreferencesPatch == nil {
		return nil, fmt.Errorf("update profile: no changes provided")
	}

	var result *vo.Profile
	err := s.txManager.WithinTx(ctx, txmanager.TxOptions{}, func(txCtx context.Context, sess txmanager.Session) error {
		record, err := s.repo.Get(txCtx, sess, input.UserID)
		if err != nil && !errors.Is(err, repositories.ErrProfileUserNotFound) {
			return fmt.Errorf("load profile: %w", err)
		}

		prefs := map[string]any{}
		currentVersion := int64(0)
		if err == nil {
			prefs = record.PreferencesJSON
			currentVersion = record.ProfileVersion
			if input.ExpectedVersion != nil && *input.ExpectedVersion != currentVersion {
				return ErrProfileVersionConflict
			}
		} else {
			if input.ExpectedVersion != nil {
				return ErrProfileVersionConflict
			}
		}

		prefs = ensurePrefs(prefs)
		updatePreferences(prefs, input.PreferencesPatch)

		displayName := valueOrDefault(input.DisplayName, "")
		if displayName == "" {
			if record == nil {
				return fmt.Errorf("update profile: display_name required for creation")
			}
			displayName = record.DisplayName
		}

		nextVersion := currentVersion + 1
		avatar := input.AvatarURL
		if avatar == nil && record != nil {
			avatar = record.AvatarURL
		}

		upsertInput := repositories.UpsertProfileUserInput{
			UserID:         input.UserID,
			DisplayName:    displayName,
			AvatarURL:      avatar,
			ProfileVersion: nextVersion,
			Preferences:    prefs,
		}

		recordUpdated, err := s.repo.Upsert(txCtx, sess, upsertInput)
		if err != nil {
			return err
		}
		result = vo.NewProfileFromPO(recordUpdated, toPreferencesVO(recordUpdated.PreferencesJSON))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// UpdatePreferencesInput 描述偏好字段更新参数。
type UpdatePreferencesInput struct {
	UserID          uuid.UUID
	LearningGoal    *string
	DailyQuotaMins  *int32
	Extra           map[string]any
	ExpectedVersion *int64
}

// UpdatePreferences 局部更新偏好字段。
func (s *ProfileService) UpdatePreferences(ctx context.Context, input UpdatePreferencesInput) (*vo.Profile, error) {
	if input.LearningGoal == nil && input.DailyQuotaMins == nil && len(input.Extra) == 0 {
		return nil, fmt.Errorf("update preferences: no fields provided")
	}

	var result *vo.Profile
	err := s.txManager.WithinTx(ctx, txmanager.TxOptions{}, func(txCtx context.Context, sess txmanager.Session) error {
		record, err := s.repo.Get(txCtx, sess, input.UserID)
		if err != nil {
			if errors.Is(err, repositories.ErrProfileUserNotFound) {
				return ErrProfileNotFound
			}
			return fmt.Errorf("load profile: %w", err)
		}

		if input.ExpectedVersion != nil && *input.ExpectedVersion != record.ProfileVersion {
			return ErrProfileVersionConflict
		}

		prefs := ensurePrefs(record.PreferencesJSON)
		if input.LearningGoal != nil {
			prefs["learning_goal"] = *input.LearningGoal
		}
		if input.DailyQuotaMins != nil {
			prefs["daily_quota_minutes"] = *input.DailyQuotaMins
		}
		for k, v := range input.Extra {
			prefs[k] = v
		}

		upsertInput := repositories.UpsertProfileUserInput{
			UserID:         input.UserID,
			DisplayName:    record.DisplayName,
			AvatarURL:      record.AvatarURL,
			ProfileVersion: record.ProfileVersion + 1,
			Preferences:    prefs,
		}

		recordUpdated, err := s.repo.Upsert(txCtx, sess, upsertInput)
		if err != nil {
			return err
		}
		result = vo.NewProfileFromPO(recordUpdated, toPreferencesVO(recordUpdated.PreferencesJSON))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func ensurePrefs(prefs map[string]any) map[string]any {
	if prefs == nil {
		return map[string]any{}
	}
	return prefs
}

func updatePreferences(prefs map[string]any, patch *vo.Preferences) {
	if patch == nil {
		return
	}
	if patch.LearningGoal != nil {
		prefs["learning_goal"] = *patch.LearningGoal
	}
	if patch.DailyQuotaMinutes != nil {
		prefs["daily_quota_minutes"] = *patch.DailyQuotaMinutes
	}
	if len(patch.Extra) > 0 {
		for k, v := range patch.Extra {
			prefs[k] = v
		}
	}
}

func toPreferencesVO(data map[string]any) vo.Preferences {
	prefs := vo.Preferences{Extra: map[string]any{}}
	if data == nil {
		return prefs
	}
	if lg, ok := data["learning_goal"].(string); ok && lg != "" {
		prefs.LearningGoal = &lg
	}
	if quota, ok := castToInt32(data["daily_quota_minutes"]); ok {
		prefs.DailyQuotaMinutes = &quota
	}
	for k, v := range data {
		if k == "learning_goal" || k == "daily_quota_minutes" {
			continue
		}
		prefs.Extra[k] = v
	}
	return prefs
}

func castToInt32(val any) (int32, bool) {
	switch v := val.(type) {
	case int:
		return int32(v), true
	case int32:
		return v, true
	case int64:
		return int32(v), true
	case float64:
		return int32(v), true
	default:
		return 0, false
	}
}

func valueOrDefault(ptr *string, fallback string) string {
	if ptr != nil {
		return *ptr
	}
	return fallback
}
