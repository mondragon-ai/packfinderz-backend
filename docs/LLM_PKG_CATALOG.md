## pkg/auth
- `MintAccessToken(cfg config.JWTConfig, now time.Time, payload AccessTokenPayload) (string, error)` validates JWT params plus roles/store/KYC before signing HS256 tokens with the configured expiration (pkg/auth/token.go:15-119).
- `ParseAccessToken` and `ParseAccessTokenAllowExpired` return typed `AccessTokenClaims` used by middleware and refresh operations (pkg/auth/token.go:66-119).
- `AccessTokenPayload`/`AccessTokenClaims` carry user, active store, role, store type, KYC, and JTI metadata for minted tokens (pkg/auth/claims.go:9-27).

## pkg/auth/session
- `Manager` (`NewManager`, `Generate`, `Rotate`, `Revoke`, `HasSession`) maps access IDs to refresh tokens in Redis, enforces TTLs, and rotates tokens via constant-time comparison (pkg/auth/session/manager.go:45-154).

## pkg/config
- `Config` aggregates `App`, `Service`, `DB`, `Redis`, `JWT`, `Password`, feature flags, `Eventing.OutboxIdempotencyTTL`, and GCP/GCS/PubSub/Outbox settings loaded via `Load()` and `envconfig` (pkg/config/config.go:12-234).

## pkg/db
- `New(ctx, cfg, logg)` builds a GORM/Postgres client, applies pool settings, and exposes `DB()`, `Ping()`, `Close()`, `Exec()`, `Raw()`, and `WithTx()` helpers for transactional work (pkg/db/client.go:17-136).

## pkg/redis
- `Client` (`New`, `Set`, `Get`, `SetNX`, `Incr`, `IncrWithTTL`, `FixedWindowAllow`) unifies redis commands, key namespaces (`IdempotencyKey`, `RateLimitKey`, `AccessSessionKey`) and refresh-token helpers for session handling (pkg/redis/client.go:33-233).

## pkg/pubsub
- `Client` (`NewClient`, `Subscription`, `MediaSubscription`, `DomainPublisher`, `Ping`) boots a V2 client, verifies the configured subscriptions/topics exist, and exposes publishers/subscribers (pkg/pubsub/client.go:18-202).

## pkg/storage/gcs
- `Client` loads credentials (JSON/service account/metadata), keeps a cached token source, pings the bucket, and exposes `SignedURL`, `SignedReadURL`, `DeleteObject`, and bucket helpers that embed service-account signing logic (pkg/storage/gcs/client.go:35-506).

## pkg/outbox
- `ActorRef` + `PayloadEnvelope` describe stored envelopes that wrap `DomainEvent.Data` with version, event ID, actor, and timestamps before persistence (pkg/outbox/envelope.go:9-21).
- `DomainEvent` carries aggregate/type/actor/data metadata and `Service.Emit(ctx, tx, event)` marshals it into `OutboxEvent` rows while logging the queued event (pkg/outbox/service.go:1-98).
- `Repository` supports `Insert`, `FetchUnpublishedForPublish`, `MarkPublishedTx`, and `MarkFailedTx`, handling locking, attempt counts, and truncated `last_error` fields (pkg/outbox/repository.go:20-101).
- `DecoderRegistry` lets consumers register versioned decoders for published payloads (pkg/outbox/registry.go:1-32).

## pkg/outbox/idempotency
- `Manager` (`NewManager`, `CheckAndMarkProcessed`) wraps a `redis.IdempotencyStore`, enforces `PACKFINDERZ_EVENTING_IDEMPOTENCY_TTL` (default 720h), and leans on `pf:idempotency:evt:processed:<consumer>:<event_id>` keys (pkg/outbox/idempotency/idempotency.go:1-66; pkg/config/config.go:131-181).

## pkg/migrate
- `MaybeRunDev` auto-applies migrations in dev mode, while `Run` and `MigrateToVersion` delegate to `goose` for CLI migrations and version targeting (pkg/migrate/autorun.go:12-34; pkg/migrate/migrate.go:12-72).

## pkg/pagination
- `Params`, `Cursor`, `NormalizeLimit`, `LimitWithBuffer`, `EncodeCursor`, and `ParseCursor` encapsulate the cursor pagination contract used by licenses/media listings (pkg/pagination/pagination.go:12-80).

## pkg/security
- `HashPassword`, `VerifyPassword`, and `GenerateTempPassword` wrap Argon2id hashing and random-password generation tuned by `PasswordConfig`, and validate hash formats (pkg/security/password.go:15-166).

## pkg/types
- `Address`, `Social`, `GeographyPoint`, and `Ratings` mirror Postgres composite types (`address_t`, `social_t`, geography, JSONB) with `Value`/`Scan` helpers used by GORM models (pkg/types/address.go:10-109; pkg/types/social.go:9-58; pkg/types/geography_point.go:12-117; pkg/types/ratings.go:9-47).

## internal/auth
- `Service.Login(ctx, LoginRequest)` returns `LoginResponse` with tokens, user DTO, and `StoreSummary` list after verifying credentials and membership (internal/auth/service.go:24-153; internal/auth/dto.go:9-29).
- `RegisterService.Register(ctx, RegisterRequest)` builds user/store/membership rows under a transaction, hashing passwords and enforcing TOS/store type validation (internal/auth/register.go:21-133).
- `SwitchStoreService.Switch(ctx, SwitchStoreInput)` verifies membership status, rotates refresh tokens, and mints a store-scoped access token (internal/auth/switch_store.go:18-118).

## internal/memberships
- `Repository` exposes `ListUserStores`, `GetMembershipWithStore`, `ListStoreUsers`, `CreateMembership`, `UserHasRole`, `CountMembersWithRoles`, and `DeleteMembership` to mediate memberships (internal/memberships/repo.go:13-145).
- DTOs `MembershipWithStore` and `StoreUserDTO` blend membership metadata with store/user details for API responses (internal/memberships/dto.go:12-76).

## internal/stores
- `Service` (`GetByID`, `Update`, `ListUsers`, `InviteUser`, `RemoveUser`) ties `stores.Repository`, `memberships.Repository`, and `users.Repository` to enforce role checks, update fields, invite users, and protect the last owner (internal/stores/service.go:42-373).
- `StoreDTO`, `CreateStoreDTO`, and model mappers shape the safe tenant payload returned to clients (internal/stores/dto.go:13-140).

## internal/users
- `Repository` provides `Create`, `FindByEmail`, `FindByID`, `UpdateLastLogin`, `UpdateStoreIDs`, and `UpdatePasswordHash`, while `UserDTO` hides credentials (internal/users/repo.go:12-70; internal/users/dto.go:11-78).

## internal/media
- `Service` operations `PresignUpload`, `ListMedia`, `DeleteMedia`, and `GenerateReadURL` validate roles, enforce mime/kind rules, persist `Media` rows, and sign URLs via GCS (internal/media/service.go:39-332; internal/media/list.go:15-139).
- `PresignInput`, `PresignOutput`, `ListParams`, `ListResult`, `ListItem`, `ReadURLParams`, `ReadURLOutput`, and `DeleteMediaParams` define the request/response contracts (internal/media/service.go:94-244; internal/media/list.go:15-139).
- `Repository` supports `Create`, `FindByID`, `List`, `MarkUploaded`, and `MarkDeleted` for metadata lifecycle updates (internal/media/repo.go:14-110).
- `consumer.Consumer.Run` processes GCS `OBJECT_FINALIZE` notifications, looks up media by GCS key, and calls `MarkUploaded` with retries/nacks for transient DB errors (internal/media/consumer/consumer.go:30-235).

## internal/licenses
- `Service` `CreateLicense` validates role/media ownership/mime, persists `License`, and `ListLicenses` applies cursor pagination plus signed downloads (internal/licenses/service.go:18-224).
- `CreateLicenseInput`, `ListParams`, `ListResult`, and `ListItem` describe license creation/listing payloads (internal/licenses/service.go:51-213; internal/licenses/list.go:12-59).
- `Repository` `Create` and `List` wrap GORM operations for license rows (internal/licenses/repo.go:10-43).
