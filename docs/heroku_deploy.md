# Heroku Release & Deploy Checklist

## Release Notes

* `heroku.yml` now wires up the API (`web`) and worker dynos to the compiled binaries (`./bin/api` and `./bin/worker`).
* Release automation still relies on the existing `cmd/migrate` binary so schema changes remain manual in production.

## Deploy Checklist

1. **Refresh infra credentials** (Cloud SQL, Redis, Pub/Sub emulator if applicable) and ensure environment variables match the target (prod/dev) before kicking off a release.
2. **Run local checks**: `gofmt -l .`, `golangci-lint run ./...`, `go test ./...`, and `go build ./cmd/api ./cmd/worker ./cmd/migrate` to match what CI already enforces.
3. **Migrations**: in production run `go run ./cmd/migrate up` (or `make migrate-up`) manually against the target Postgres; in dev you may rely on the auto-migrate feature (`PACKFINDERZ_APP_ENV=dev` and `PACKFINDERZ_AUTO_MIGRATE=true`) but double-check the auto-run logs before deploy.
4. **Deploy**: push to Heroku (`git push heroku main` or `heroku container:release`) so the buildpack emits `./bin/api` and `./bin/worker` used by `heroku.yml`.
5. **Post-deploy verification**:
   * Check `/health/ready` to confirm Postgres and Redis are healthy.
   * Verify logs for the API and worker dynos to ensure migrations (if any) and startup completed without errors.
   * Exercise critical HTTP routes (search, checkout) and monitor agent heartbeat logs to detect issues early.
6. **Document release**: note the version, migration steps, and any sticky issues in the release notes channel so follow-up teams know what to watch.

## Hybrid Migration Policy

* **Production**: all migrations run manually through `cmd/migrate` (or `make migrate-up`). Do **not** enable `PACKFINDERZ_AUTO_MIGRATE` in prod.
* **Development**: setting `PACKFINDERZ_APP_ENV=dev` and `PACKFINDERZ_AUTO_MIGRATE=true` permits the API/worker to auto-run migrations at startup, but teams should still run `cmd/migrate up` locally before deployment to keep parity.

## Readiness Expectations

* The `/health/ready` endpoint must report both Postgres and Redis as healthy before traffic routing occurs.
* If a Heroku dyno fails readiness, inspect configuration (DB URI, Cloud SQL Proxy, gRPC endpoints) and redeploy after fixing the gate.

