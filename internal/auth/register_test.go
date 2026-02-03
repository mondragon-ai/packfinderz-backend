package auth

import (
	"context"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/internal/users"
	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	pkgmodels "github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/security"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type stubTxRunner struct{}

func (s stubTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}

type stubUserRepository struct {
	data      map[string]*pkgmodels.User
	created   *pkgmodels.User
	createErr error
}

func newStubUserRepository() *stubUserRepository {
	return &stubUserRepository{data: map[string]*pkgmodels.User{}}
}

func (s *stubUserRepository) FindByEmail(ctx context.Context, email string) (*pkgmodels.User, error) {
	if user, ok := s.data[email]; ok {
		return user, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *stubUserRepository) Create(ctx context.Context, dto users.CreateUserDTO) (*pkgmodels.User, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	user := &pkgmodels.User{
		ID:           uuid.New(),
		Email:        dto.Email,
		FirstName:    dto.FirstName,
		LastName:     dto.LastName,
		PasswordHash: dto.PasswordHash,
		Phone:        dto.Phone,
	}
	if dto.IsActive != nil {
		user.IsActive = *dto.IsActive
	} else {
		user.IsActive = true
	}
	s.data[dto.Email] = user
	s.created = user
	return user, nil
}

type stubStoreRepository struct {
	created *pkgmodels.Store
}

func (s *stubStoreRepository) Create(ctx context.Context, dto stores.CreateStoreDTO) (*pkgmodels.Store, error) {
	store := dto.ToModel()
	store.ID = uuid.New()
	s.created = store
	return store, nil
}

type stubMembershipRepository struct {
	calledWith struct {
		storeID uuid.UUID
		userID  uuid.UUID
		role    enums.MemberRole
		status  enums.MembershipStatus
	}
	err error
}

func (s *stubMembershipRepository) CreateMembership(ctx context.Context, storeID, userID uuid.UUID, role enums.MemberRole, invitedBy *uuid.UUID, status enums.MembershipStatus) (*pkgmodels.StoreMembership, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.calledWith.storeID = storeID
	s.calledWith.userID = userID
	s.calledWith.role = role
	s.calledWith.status = status
	return &pkgmodels.StoreMembership{
		StoreID: storeID,
		UserID:  userID,
		Role:    role,
		Status:  status,
	}, nil
}

type registerTestSetup struct {
	service    RegisterService
	userRepo   *stubUserRepository
	storeRepo  *stubStoreRepository
	memberRepo *stubMembershipRepository
}

func newRegisterTestSetup(t *testing.T) *registerTestSetup {
	t.Helper()
	userRepo := newStubUserRepository()
	storeRepo := &stubStoreRepository{}
	memberRepo := &stubMembershipRepository{}
	svc, err := NewRegisterService(RegisterServiceParams{
		TxRunner: stubTxRunner{},
		UserRepoFactory: func(tx *gorm.DB) registerUserRepository {
			return userRepo
		},
		StoreRepoFactory: func(tx *gorm.DB) registerStoreRepository {
			return storeRepo
		},
		MembershipRepoFactory: func(tx *gorm.DB) registerMembershipRepository {
			return memberRepo
		},
		PasswordConfig: config.PasswordConfig{},
	})
	if err != nil {
		t.Fatalf("new register service: %v", err)
	}
	return &registerTestSetup{
		service:    svc,
		userRepo:   userRepo,
		storeRepo:  storeRepo,
		memberRepo: memberRepo,
	}
}

func sampleRegisterRequest(email, company string) RegisterRequest {
	return RegisterRequest{
		FirstName:   "Jamie",
		LastName:    "Rivera",
		Email:       email,
		Password:    "Secret123!",
		CompanyName: company,
		StoreType:   enums.StoreTypeBuyer,
		Address: types.Address{
			Line1:      "123 Main St",
			City:       "Oklahoma City",
			State:      "OK",
			PostalCode: "73102",
			Country:    "US",
			Lat:        35.4676,
			Lng:        -97.5164,
		},
		AcceptTOS: true,
	}
}

func TestRegisterCreatesStoreForNewUser(t *testing.T) {
	setup := newRegisterTestSetup(t)
	req := sampleRegisterRequest("new@example.com", "NewCo")

	if err := setup.service.Register(context.Background(), req); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if setup.userRepo.created == nil {
		t.Fatalf("expected user to be created")
	}
	if setup.storeRepo.created == nil {
		t.Fatalf("expected store to be created")
	}
	if setup.memberRepo.calledWith.storeID != setup.storeRepo.created.ID {
		t.Fatalf("membership not linked to created store")
	}
	if setup.memberRepo.calledWith.userID != setup.userRepo.created.ID {
		t.Fatalf("membership not linked to created user")
	}
}

func TestRegisterCreatesStoreForExistingUser(t *testing.T) {
	setup := newRegisterTestSetup(t)
	password := "Secret123!"
	hash, err := security.HashPassword(password, config.PasswordConfig{})
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := &pkgmodels.User{
		ID:           uuid.New(),
		Email:        "existing@example.com",
		FirstName:    "Existing",
		LastName:     "User",
		PasswordHash: hash,
		IsActive:     true,
	}
	setup.userRepo.data[user.Email] = user

	req := sampleRegisterRequest(user.Email, "SecondCo")
	req.Password = password

	if err := setup.service.Register(context.Background(), req); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if setup.userRepo.created != nil {
		t.Fatalf("expected no new user creation")
	}
	if setup.storeRepo.created == nil {
		t.Fatalf("expected store to be created")
	}
	if setup.storeRepo.created.OwnerID != user.ID {
		t.Fatalf("store owner mismatch")
	}
	if setup.memberRepo.calledWith.userID != user.ID {
		t.Fatalf("membership not linked to existing user")
	}
}
