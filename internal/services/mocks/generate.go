package mocks

//go:generate go run github.com/golang/mock/mockgen -destination=mock_video_projection_repository.go -package=mocks github.com/bionicotaku/lingo-services-profile/internal/services VideoProjectionRepository
//go:generate go run github.com/golang/mock/mockgen -destination=mock_profile_users_repository.go -package=mocks github.com/bionicotaku/lingo-services-profile/internal/services ProfileUsersRepository
