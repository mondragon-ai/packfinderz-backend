# Internal + Model Reference

## Table of contents
1. `internal/auth` – auth flows (login, register, store switch) + helpers.
2. `internal/media` – media presign repo/service + Pub/Sub consumer.
3. `internal/memberships` – membership DTOs/mappers + repo helpers.
4. `internal/stores` – tenant management + invite/remove helpers.
5. `internal/users` – user DTOs/repos supporting auth flows.
6. `pkg/db/models` – canonical GORM structs for users, stores, media.

## internal/auth
- **Descriptor:** login + onboarding + store switching with DTOs + JWT/session helpers.

### Helpers & resources
- `LoginRequest`, `LoginResponse`, `StoreSummary` (`internal/auth/dto.go`).
- `RegisterService.Register` (`internal/auth/register.go`) – tokens holder; validates input, hashes passwords, creates user + store + owner membership, enforces TOS + store type.
- `Service.Login` (`internal/auth/service.go`) – verifies password, loads stores, updates last login, mints access/refresh tokens via `pkg/auth` + `session`, returns `users.FromModel`.
- `SwitchStoreService.Switch` (`internal/auth/switch_store.go`) – validates membership, rotates sessions, mints JWT scoped to new store, returns refreshed tokens + `StoreSummary`.

## internal/media
- **Descriptor:** manages GCS presign + marks uploads ready via Pub/Sub events.

### Helpers & resources
- `Repository` (`internal/media/repo.go`) – CRUD helpers for `models.Media`, mark uploaded, find by GCS key.
- `Service.PresignUpload` (`internal/media/service.go`) – validates roles/mime/size, creates pending media row, requests signed PUT URL, reverts on failure, exposes `PresignInput/Output`.
- `consumer.NewConsumer` + `Consumer.Run` (`internal/media/consumer/consumer.go`) – listens to GCS OBJECT_FINALIZE, decodes payload, idempotently looks up `Media`, logs fields, calls `MarkUploaded`, handles retryable DB errors.
- helpers: `isAllowedMime`, `buildGCSKey`, `parseAttributes`, `firstNonEmpty`, `gcsBucket`.

## internal/memberships
- **Descriptor:** DTOs + repo helpers linking users to stores with roles/status.

### Helpers & resources
- DTOs (`internal/memberships/dto.go`): `MembershipDTO`, `MembershipWithStore`, `StoreUserDTO`, mapper `ToDTO`, `copyUUIDPointer`.
- Mappers (`internal/memberships/mapper.go`): conversions from custom rows to DTOs (`membershipWithStoreFromRow`, `storeUsersFromRows`, etc.).
- `Repository` (`internal/memberships/repo.go`): `ListUserStores`, `ListStoreUsers`, `GetMembership`, `CreateMembership`, `DeleteMembership`, `UserHasRole`, `CountMembersWithRoles`, `GetMembershipWithStore` (with GORM joins on `stores`/`users`).

## internal/stores
- **Descriptor:** tenant service for updates, user invites/removals, and store DTO helpers.

### Helpers & resources
- DTOs (`internal/stores/dto.go`): `StoreDTO`, `CreateStoreDTO`, `FromModel`, `CreateStoreDTO.ToModel`.
- `Service` (`internal/stores/service.go`):
  - `GetByID` + `Update` – enforces owner/manager roles, clones inputs, updates `Store` via repo.
  - `ListUsers` – wraps `memberships.ListStoreUsers` with RBAC guard.
  - `InviteUser` – validates invite, reuses or creates user, resets password if needed, creates membership, returns `StoreUserDTO` + temp password.
  - `RemoveUser` – RBAC guard, prevents deleting last owner, deletes membership.
- Helpers: `createNewUser`, `resetUserPassword`, `fetchStoreUser`, `cloneStringPtr`, `cloneSocial`, `cloneRatings`, `cloneCategories`.

## internal/users
- **Descriptor:** user DTOs/repos backing auth + store flows.

### Helpers & resources
- DTOs (`internal/users/dto.go`): `UserDTO`, `CreateUserDTO`, conversions `FromModel`, `CreateUserDTO.ToModel`.
- `Repository` (`internal/users/repo.go`): `Create`, `FindByEmail`, `FindByID`, `UpdateLastLogin`, `UpdateStoreIDs`, `UpdatePasswordHash`.

## pkg/db/models
- **Descriptor:** canonical Postgres tables mapped via GORM.

### Models
- `models.User` (`pkg/db/models/user.go`): identity table with email, password hash, system role, `store_ids` array, login timestamps.
- `models.Store` (`pkg/db/models/store.go`): tenant data (type, address/geom, social links, ratings/categories arrays, owner reference, KYC/subscription flags, timestamps).
- `models.StoreMembership` (`pkg/db/models/store_membership.go`): join table capturing `role`, `status`, invited-by metadata.
- `models.Media` (`pkg/db/models/media.go`): upload metadata (kind/status, GCS key, OCR/verification timestamps, size/flags).
