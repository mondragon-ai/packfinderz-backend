package stores

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	"github.com/angelmondragon/packfinderz-backend/internal/users"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func newStoreService(repo storeRepository, members stubMembershipsRepo, usersRepo usersRepository) (Service, error) {
	svc, _, err := newStoreServiceWithAttachmentStub(repo, members, usersRepo, nil, nil, nil)
	return svc, err
}

func newStoreServiceWithAttachmentStub(repo storeRepository, members stubMembershipsRepo, usersRepo usersRepository, reconciler *stubAttachmentReconciler, mediaRepo mediaLookup, licenseRepo licenseRepository) (Service, *stubAttachmentReconciler, error) {
	if reconciler == nil {
		reconciler = &stubAttachmentReconciler{}
	}
	if mediaRepo == nil {
		storeID := uuid.Nil
		if sr, ok := repo.(*stubStoreRepo); ok && sr.store != nil {
			storeID = sr.store.ID
		}
		defaultMedia := &models.Media{
			ID:        uuid.New(),
			StoreID:   storeID,
			Status:    enums.MediaStatusUploaded,
			PublicURL: "https://example.com/default",
		}
		mediaRepo = &stubMediaRepo{
			defaultMedia: defaultMedia,
		}
	}
	if licenseRepo == nil {
		licenseRepo = &stubLicenseRepo{}
	}
	svc, err := NewService(ServiceParams{
		Repo:                 repo,
		Memberships:          members,
		Users:                usersRepo,
		PasswordCfg:          config.PasswordConfig{},
		TransactionRunner:    stubTxRunner{},
		AttachmentReconciler: reconciler,
		MediaRepo:            mediaRepo,
		LicenseRepo:          licenseRepo,
	})
	return svc, reconciler, err
}

func TestNewServiceRequiresRepo(t *testing.T) {
	_, err := NewService(ServiceParams{
		Repo:                 nil,
		Memberships:          stubMembershipsRepo{},
		Users:                &stubUsersRepo{},
		PasswordCfg:          config.PasswordConfig{},
		TransactionRunner:    stubTxRunner{},
		AttachmentReconciler: &stubAttachmentReconciler{},
	})
	if err == nil {
		t.Fatal("expected error creating service without repo")
	}
}

func TestNewServiceRequiresMembershipRepo(t *testing.T) {
	_, err := NewService(ServiceParams{
		Repo:                 &stubStoreRepo{},
		Memberships:          nil,
		Users:                &stubUsersRepo{},
		PasswordCfg:          config.PasswordConfig{},
		TransactionRunner:    stubTxRunner{},
		AttachmentReconciler: &stubAttachmentReconciler{},
	})
	if err == nil {
		t.Fatal("expected error creating service without memberships repo")
	}
}

func TestServiceGetByIDSuccess(t *testing.T) {
	store := baseStore()
	repo := &stubStoreRepo{store: store}
	svc, err := newStoreService(repo, stubMembershipsRepo{allowed: true}, &stubUsersRepo{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	dto, err := svc.GetByID(context.Background(), store.ID)
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	if dto.ID != store.ID {
		t.Fatalf("expected id %s got %s", store.ID, dto.ID)
	}
	if dto.CompanyName != store.CompanyName {
		t.Fatalf("expected company name %s got %s", store.CompanyName, dto.CompanyName)
	}
	if dto.Phone == nil || *dto.Phone != *store.Phone {
		t.Fatalf("expected phone %q got %v", *store.Phone, dto.Phone)
	}
	if dto.Address.Line1 != store.Address.Line1 {
		t.Fatalf("address mismatch: expected %s got %s", store.Address.Line1, dto.Address.Line1)
	}
}

func TestServiceGetByIDNotFound(t *testing.T) {
	repo := &stubStoreRepo{err: gorm.ErrRecordNotFound}
	svc, err := newStoreService(repo, stubMembershipsRepo{allowed: true}, &stubUsersRepo{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, gotErr := svc.GetByID(context.Background(), uuid.New())
	if gotErr == nil {
		t.Fatal("expected error")
	}
	if typed := pkgerrors.As(gotErr); typed == nil || typed.Code() != pkgerrors.CodeNotFound {
		t.Fatalf("expected not found code, got %v", gotErr)
	}
}

func TestServiceGetByIDDependencyError(t *testing.T) {
	repo := &stubStoreRepo{err: errors.New("boom")}
	svc, err := newStoreService(repo, stubMembershipsRepo{allowed: true}, &stubUsersRepo{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, gotErr := svc.GetByID(context.Background(), uuid.New())
	if gotErr == nil {
		t.Fatal("expected error")
	}
	if typed := pkgerrors.As(gotErr); typed == nil || typed.Code() != pkgerrors.CodeDependency {
		t.Fatalf("expected dependency error, got %v", gotErr)
	}
}

func TestServiceGetManagerViewIncludesOwnerAndLicenses(t *testing.T) {
	store := baseStore()
	repo := &stubStoreRepo{store: store}
	user := &models.User{
		ID:        store.OwnerID,
		FirstName: "Owner",
		LastName:  "Keeper",
		Email:     "owner@example.com",
		LastLoginAt: func() *time.Time {
			t := time.Now().UTC()
			return &t
		}(),
	}
	licenseRepo := &stubLicenseRepo{
		list: []models.License{
			{Number: "LIC-001", Type: enums.LicenseTypeProducer},
			{Number: "LIC-002", Type: enums.LicenseTypeMerchant},
		},
	}
	membershipsRepo := stubMembershipsRepo{
		allowed: true,
		existingMembership: &models.StoreMembership{
			StoreID: store.ID,
			UserID:  user.ID,
			Role:    enums.MemberRoleOwner,
		},
	}
	usersRepo := &stubUsersRepo{
		user: user,
		byID: user,
	}
	svc, _, err := newStoreServiceWithAttachmentStub(repo, membershipsRepo, usersRepo, nil, nil, licenseRepo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	dto, err := svc.GetManagerView(context.Background(), store.ID)
	if err != nil {
		t.Fatalf("get manager view: %v", err)
	}
	if dto.Owner.Email != user.Email {
		t.Fatalf("expected owner email %s got %s", user.Email, dto.Owner.Email)
	}
	if dto.Owner.Role == nil || *dto.Owner.Role != enums.MemberRoleOwner.String() {
		t.Fatalf("expected owner role owner got %v", dto.Owner.Role)
	}
	if len(dto.Licenses) != 2 {
		t.Fatalf("expected 2 licenses got %d", len(dto.Licenses))
	}
	if dto.Licenses[0].Number != "LIC-001" {
		t.Fatalf("unexpected license number %s", dto.Licenses[0].Number)
	}
}

func TestServiceGetManagerViewNoLicenses(t *testing.T) {
	store := baseStore()
	repo := &stubStoreRepo{store: store}
	user := &models.User{
		ID:        store.OwnerID,
		FirstName: "Owner",
		LastName:  "Keeper",
		Email:     "owner@example.com",
	}
	usersRepo := &stubUsersRepo{
		user: user,
		byID: user,
	}
	membershipsRepo := stubMembershipsRepo{
		allowed: true,
	}
	svc, _, err := newStoreServiceWithAttachmentStub(repo, membershipsRepo, usersRepo, nil, nil, &stubLicenseRepo{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	dto, err := svc.GetManagerView(context.Background(), store.ID)
	if err != nil {
		t.Fatalf("get manager view: %v", err)
	}
	if len(dto.Licenses) != 0 {
		t.Fatalf("expected empty licenses slice got %v", dto.Licenses)
	}
}

func TestServiceGetManagerViewWithoutMembershipRole(t *testing.T) {
	store := baseStore()
	repo := &stubStoreRepo{store: store}
	user := &models.User{
		ID:        store.OwnerID,
		FirstName: "Owner",
		LastName:  "Keeper",
		Email:     "owner@example.com",
	}
	usersRepo := &stubUsersRepo{
		user: user,
		byID: user,
	}
	svc, _, err := newStoreServiceWithAttachmentStub(repo, stubMembershipsRepo{allowed: true}, usersRepo, nil, nil, &stubLicenseRepo{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	dto, err := svc.GetManagerView(context.Background(), store.ID)
	if err != nil {
		t.Fatalf("get manager view: %v", err)
	}
	if dto.Owner.Role != nil {
		t.Fatalf("expected nil owner role got %v", dto.Owner.Role)
	}
}

func TestServiceUpdateSuccess(t *testing.T) {
	store := baseStore()
	repo := &stubStoreRepo{store: store}
	att := &stubAttachmentReconciler{}
	svc, _, err := newStoreServiceWithAttachmentStub(repo, stubMembershipsRepo{allowed: true}, &stubUsersRepo{}, att, nil, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	newDescription := "new description"
	newBanner := "http://banner"
	newRatings := map[string]int{"quality": 5}
	newCategories := []string{"flower", "edibles"}
	logoID := uuid.New()
	input := UpdateStoreInput{
		CompanyName: stringPtr("Updated Store"),
		Description: &newDescription,
		BannerURL:   &newBanner,
		Ratings:     &newRatings,
		Categories:  &newCategories,
		LogoMediaID: types.NullableUUID{Valid: true, Value: &logoID},
	}

	dto, err := svc.Update(context.Background(), uuid.New(), store.ID, input)
	if err != nil {
		t.Fatalf("update store: %v", err)
	}
	if dto.CompanyName != "Updated Store" {
		t.Fatalf("expected company name updated, got %s", dto.CompanyName)
	}
	if dto.Description == nil || *dto.Description != newDescription {
		t.Fatalf("expected description %q got %v", newDescription, dto.Description)
	}
	if dto.BannerURL == nil || *dto.BannerURL != newBanner {
		t.Fatalf("expected banner %q got %v", newBanner, dto.BannerURL)
	}
	if dto.Ratings["quality"] != 5 {
		t.Fatalf("expected rating quality=5 got %v", dto.Ratings)
	}
	if len(dto.Categories) != 2 {
		t.Fatalf("expected categories updated got %v", dto.Categories)
	}
	if len(att.calls) != 2 {
		t.Fatalf("expected 2 attachment reconciliations got %d", len(att.calls))
	}
	logoCall := att.calls[0]
	if logoCall.entityType != models.AttachmentEntityStoreLogo {
		t.Fatalf("expected logo reconciler call got %s", logoCall.entityType)
	}
	if logoCall.storeID != store.ID {
		t.Fatalf("expected store id %s got %s", store.ID, logoCall.storeID)
	}
	if len(logoCall.newIDs) != 1 || logoCall.newIDs[0] != logoID {
		t.Fatalf("expected logo new id %s got %v", logoID, logoCall.newIDs)
	}
	bannerCall := att.calls[1]
	if bannerCall.entityType != models.AttachmentEntityStoreBanner {
		t.Fatalf("expected banner reconciler call got %s", bannerCall.entityType)
	}
	if len(bannerCall.newIDs) != 0 {
		t.Fatalf("expected no banner changes got %v", bannerCall.newIDs)
	}
}

func TestServiceUpdatePopulatesMediaURLs(t *testing.T) {
	store := baseStore()
	repo := &stubStoreRepo{store: store}
	att := &stubAttachmentReconciler{}
	logoID := uuid.New()
	bannerID := uuid.New()
	mediaRepo := &stubMediaRepo{
		entries: map[uuid.UUID]*models.Media{
			logoID: {
				ID:        logoID,
				StoreID:   store.ID,
				Status:    enums.MediaStatusUploaded,
				PublicURL: "https://logo.example/logo.png",
			},
			bannerID: {
				ID:        bannerID,
				StoreID:   store.ID,
				Status:    enums.MediaStatusUploaded,
				PublicURL: "https://banner.example/banner.png",
			},
		},
	}
	svc, _, err := newStoreServiceWithAttachmentStub(repo, stubMembershipsRepo{allowed: true}, &stubUsersRepo{}, att, mediaRepo, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	input := UpdateStoreInput{
		LogoMediaID:   types.NullableUUID{Valid: true, Value: &logoID},
		BannerMediaID: types.NullableUUID{Valid: true, Value: &bannerID},
	}

	dto, err := svc.Update(context.Background(), uuid.New(), store.ID, input)
	if err != nil {
		t.Fatalf("update store: %v", err)
	}
	if dto.LogoURL == nil || *dto.LogoURL != "https://logo.example/logo.png" {
		t.Fatalf("expected logo url set got %v", dto.LogoURL)
	}
	if dto.BannerURL == nil || *dto.BannerURL != "https://banner.example/banner.png" {
		t.Fatalf("expected banner url set got %v", dto.BannerURL)
	}
}

func TestServiceUpdateForbidden(t *testing.T) {
	repo := &stubStoreRepo{store: baseStore()}
	svc, err := newStoreService(repo, stubMembershipsRepo{allowed: false}, &stubUsersRepo{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, gotErr := svc.Update(context.Background(), uuid.New(), uuid.New(), UpdateStoreInput{})
	if gotErr == nil {
		t.Fatal("expected error")
	}
	if typed := pkgerrors.As(gotErr); typed == nil || typed.Code() != pkgerrors.CodeForbidden {
		t.Fatalf("expected forbidden code, got %v", gotErr)
	}
}

func TestServiceListUsersSuccess(t *testing.T) {
	repo := &stubStoreRepo{store: baseStore()}
	members := []memberships.StoreUserDTO{
		{
			MembershipID: uuid.New(),
			StoreID:      repo.store.ID,
			UserID:       uuid.New(),
			Email:        "user@example.com",
			FirstName:    "Test",
			LastName:     "User",
			Role:         enums.MemberRoleManager,
			Status:       enums.MembershipStatusActive,
			CreatedAt:    time.Now(),
		},
	}
	svc, err := newStoreService(repo, stubMembershipsRepo{allowed: true, members: members}, &stubUsersRepo{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	output, err := svc.ListUsers(context.Background(), uuid.New(), repo.store.ID)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(output) != 1 {
		t.Fatalf("expected 1 member got %d", len(output))
	}
	if output[0].Email != "user@example.com" {
		t.Fatalf("expected email user@example.com got %s", output[0].Email)
	}
}

func TestServiceListUsersForbidden(t *testing.T) {
	repo := &stubStoreRepo{store: baseStore()}
	svc, err := newStoreService(repo, stubMembershipsRepo{allowed: false}, &stubUsersRepo{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, gotErr := svc.ListUsers(context.Background(), uuid.New(), repo.store.ID)
	if gotErr == nil {
		t.Fatal("expected error")
	}
	if typed := pkgerrors.As(gotErr); typed == nil || typed.Code() != pkgerrors.CodeForbidden {
		t.Fatalf("expected forbidden code, got %v", gotErr)
	}
}

func TestServiceInviteCreatesMembership(t *testing.T) {
	repo := &stubStoreRepo{store: baseStore()}
	storeID := repo.store.ID
	newUserID := uuid.New()
	usersRepo := &stubUsersRepo{nextID: newUserID}
	member := memberships.StoreUserDTO{
		MembershipID: uuid.New(),
		StoreID:      storeID,
		UserID:       newUserID,
		Email:        "newbie@example.com",
		FirstName:    "New",
		LastName:     "User",
		Role:         enums.MemberRoleManager,
		Status:       enums.MembershipStatusActive,
		CreatedAt:    time.Now(),
	}
	membershipsRepo := stubMembershipsRepo{
		allowed: true,
		members: []memberships.StoreUserDTO{member},
	}
	svc, err := newStoreService(repo, membershipsRepo, usersRepo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	input := InviteUserInput{
		Email:     "newbie@example.com",
		FirstName: "New",
		LastName:  "User",
		Role:      enums.MemberRoleManager,
	}
	dto, temp, err := svc.InviteUser(context.Background(), uuid.New(), storeID, input)
	if err != nil {
		t.Fatalf("invite user: %v", err)
	}
	if temp == "" {
		t.Fatal("expected temporary password")
	}
	if dto.Email != "newbie@example.com" {
		t.Fatalf("unexpected email %s", dto.Email)
	}
}

func TestServiceInviteDuplicateMembership(t *testing.T) {
	repo := &stubStoreRepo{store: baseStore()}
	storeID := repo.store.ID
	user := &models.User{ID: uuid.New(), Email: "existing@example.com"}
	usersRepo := &stubUsersRepo{user: user}
	member := memberships.StoreUserDTO{
		MembershipID: uuid.New(),
		StoreID:      storeID,
		UserID:       user.ID,
		Email:        user.Email,
		FirstName:    "Existing",
		LastName:     "User",
		Role:         enums.MemberRoleManager,
		Status:       enums.MembershipStatusActive,
		CreatedAt:    time.Now(),
	}
	membershipsRepo := stubMembershipsRepo{
		allowed:            true,
		members:            []memberships.StoreUserDTO{member},
		existingMembership: &models.StoreMembership{StoreID: storeID, UserID: user.ID},
	}
	svc, err := newStoreService(repo, membershipsRepo, usersRepo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	input := InviteUserInput{
		Email:     user.Email,
		FirstName: "Existing",
		LastName:  "User",
		Role:      enums.MemberRoleManager,
	}
	dto, temp, err := svc.InviteUser(context.Background(), uuid.New(), storeID, input)
	if err != nil {
		t.Fatalf("invite duplicate: %v", err)
	}
	if temp != "" {
		t.Fatalf("expected no temp password for duplicate, got %s", temp)
	}
	if dto.Email != user.Email {
		t.Fatalf("unexpected email %s", dto.Email)
	}
}

func TestServiceRemoveUserSuccess(t *testing.T) {
	storeRepo := &stubStoreRepo{store: baseStore()}
	storeID := storeRepo.store.ID
	targetID := uuid.New()
	membershipsRepo := stubMembershipsRepo{
		allowed: true,
		existingMembership: &models.StoreMembership{
			StoreID: storeID,
			UserID:  targetID,
			Role:    enums.MemberRoleManager,
		},
		countByRole: 2,
	}
	svc, err := newStoreService(storeRepo, membershipsRepo, &stubUsersRepo{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err := svc.RemoveUser(context.Background(), uuid.New(), storeID, targetID); err != nil {
		t.Fatalf("remove user: %v", err)
	}
}

func TestServiceRemoveUserForbidden(t *testing.T) {
	storeRepo := &stubStoreRepo{store: baseStore()}
	svc, err := newStoreService(storeRepo, stubMembershipsRepo{allowed: false}, &stubUsersRepo{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = svc.RemoveUser(context.Background(), uuid.New(), storeRepo.store.ID, uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
	if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeForbidden {
		t.Fatalf("expected forbidden code, got %v", err)
	}
}

func TestServiceRemoveUserNotFound(t *testing.T) {
	storeRepo := &stubStoreRepo{store: baseStore()}
	membershipsRepo := stubMembershipsRepo{
		allowed: true,
	}
	svc, err := newStoreService(storeRepo, membershipsRepo, &stubUsersRepo{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = svc.RemoveUser(context.Background(), uuid.New(), storeRepo.store.ID, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}
	if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeNotFound {
		t.Fatalf("expected not found code, got %v", err)
	}
}

func TestServiceRemoveUserLastOwner(t *testing.T) {
	storeRepo := &stubStoreRepo{store: baseStore()}
	storeID := storeRepo.store.ID
	targetID := uuid.New()
	membershipsRepo := stubMembershipsRepo{
		allowed: true,
		existingMembership: &models.StoreMembership{
			StoreID: storeID,
			UserID:  targetID,
			Role:    enums.MemberRoleOwner,
		},
		countByRole: 1,
	}
	svc, err := newStoreService(storeRepo, membershipsRepo, &stubUsersRepo{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = svc.RemoveUser(context.Background(), uuid.New(), storeID, targetID)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if typed := pkgerrors.As(err); typed == nil || typed.Code() != pkgerrors.CodeConflict {
		t.Fatalf("expected conflict code, got %v", err)
	}
}

func baseStore() *models.Store {
	return &models.Store{
		ID:                   uuid.New(),
		Type:                 enums.StoreTypeBuyer,
		CompanyName:          "Test Store",
		KYCStatus:            enums.KYCStatusVerified,
		SubscriptionActive:   true,
		DeliveryRadiusMeters: 5000,
		Address: types.Address{
			Line1:      "123 Main St",
			City:       "Oklahoma City",
			State:      "OK",
			PostalCode: "73102",
			Country:    "US",
			Lat:        35.4676,
			Lng:        -97.5164,
		},
		OwnerID:     uuid.New(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Phone:       stringPtr("405-555-0000"),
		Email:       stringPtr("owner@example.com"),
		Description: stringPtr("flagship store"),
	}
}

type stubStoreRepo struct {
	store     *models.Store
	err       error
	updateErr error
	updated   *models.Store
}

func (s *stubStoreRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Store, error) {
	return s.store, s.err
}

func (s *stubStoreRepo) Update(ctx context.Context, store *models.Store) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updated = store
	return nil
}

func (s *stubStoreRepo) FindByIDWithTx(tx *gorm.DB, id uuid.UUID) (*models.Store, error) {
	return s.FindByID(context.Background(), id)
}

func (s *stubStoreRepo) UpdateWithTx(tx *gorm.DB, store *models.Store) error {
	return s.Update(context.Background(), store)
}

type stubMembershipsRepo struct {
	allowed            bool
	err                error
	members            []memberships.StoreUserDTO
	listErr            error
	existingMembership *models.StoreMembership
	createErr          error
	getErr             error
	deleteErr          error
	countErr           error
	countByRole        int64
}

func (s stubMembershipsRepo) UserHasRole(ctx context.Context, userID, storeID uuid.UUID, roles ...enums.MemberRole) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.allowed, nil
}

func (s stubMembershipsRepo) ListStoreUsers(ctx context.Context, storeID uuid.UUID) ([]memberships.StoreUserDTO, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.members, nil
}

func (s stubMembershipsRepo) GetMembership(ctx context.Context, userID, storeID uuid.UUID) (*models.StoreMembership, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.existingMembership != nil {
		return s.existingMembership, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (s stubMembershipsRepo) CreateMembership(ctx context.Context, storeID, userID uuid.UUID, role enums.MemberRole, invitedBy *uuid.UUID, status enums.MembershipStatus) (*models.StoreMembership, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	return &models.StoreMembership{
		ID:              uuid.New(),
		StoreID:         storeID,
		UserID:          userID,
		Role:            role,
		Status:          status,
		InvitedByUserID: invitedBy,
	}, nil
}

func (s stubMembershipsRepo) CountMembersWithRoles(ctx context.Context, storeID uuid.UUID, roles ...enums.MemberRole) (int64, error) {
	if s.countErr != nil {
		return 0, s.countErr
	}
	return s.countByRole, nil
}

func (s stubMembershipsRepo) DeleteMembership(ctx context.Context, storeID, userID uuid.UUID) error {
	return s.deleteErr
}

type stubMediaRepo struct {
	entries      map[uuid.UUID]*models.Media
	defaultMedia *models.Media
	err          error
}

func (s *stubMediaRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Media, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.entries != nil {
		if media, ok := s.entries[id]; ok {
			return media, nil
		}
	}
	if s.defaultMedia != nil {
		return s.defaultMedia, nil
	}
	return nil, gorm.ErrRecordNotFound
}

type stubLicenseRepo struct {
	list []models.License
	err  error
}

func (s *stubLicenseRepo) ListByStoreID(ctx context.Context, storeID uuid.UUID) ([]models.License, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.list, nil
}

type stubUsersRepo struct {
	user      *models.User
	findErr   error
	created   *models.User
	createErr error
	updateErr error
	nextID    uuid.UUID
	byID      *models.User
}

func (s *stubUsersRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.user != nil {
		return s.user, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *stubUsersRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.byID != nil && s.byID.ID == id {
		return s.byID, nil
	}
	if s.user != nil && s.user.ID == id {
		return s.user, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *stubUsersRepo) Create(ctx context.Context, dto users.CreateUserDTO) (*models.User, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	id := s.nextID
	if id == uuid.Nil {
		id = uuid.New()
	}
	user := &models.User{
		ID:           id,
		Email:        dto.Email,
		FirstName:    dto.FirstName,
		LastName:     dto.LastName,
		PasswordHash: dto.PasswordHash,
	}
	s.created = user
	return user, nil
}

func (s *stubUsersRepo) UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	if s.created != nil && s.created.ID == id {
		s.created.PasswordHash = hash
	}
	return nil
}

func stringPtr(s string) *string { return &s }

type stubTxRunner struct{}

func (stubTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}

type stubAttachmentReconciler struct {
	calls []attachmentCall
	err   error
}

type attachmentCall struct {
	entityType string
	entityID   uuid.UUID
	storeID    uuid.UUID
	oldIDs     []uuid.UUID
	newIDs     []uuid.UUID
}

func (s *stubAttachmentReconciler) Reconcile(ctx context.Context, tx *gorm.DB, entityType string, entityID, storeID uuid.UUID, oldIDs, newIDs []uuid.UUID) error {
	call := attachmentCall{
		entityType: entityType,
		entityID:   entityID,
		storeID:    storeID,
		oldIDs:     append([]uuid.UUID{}, oldIDs...),
		newIDs:     append([]uuid.UUID{}, newIDs...),
	}
	s.calls = append(s.calls, call)
	return s.err
}
