# Requester

Requester is a service that make http requests to 3rd-party services.

### Description

- `cmd/api/` - API server;
- `cmd/migrate/` - utility for migrations;
- `cmd/requester/` - handler for tasks on requests to 3rd-party.

Dependencies: `PostgreSQL`, `Amazon SQS`.

### Local launch

- Setting up environment variables: `make .env`.
- Running dependencies: `make deps`.
- Building: `make build`.
- DB migrations: `./bin/migrate up`.
- Tests: `make tests`.

### OpenAPI

[ogen](https://github.com/ogen-go/ogen) is required for generation API code from [openapi.yml](api/openapi.yml).

Updating generated code: `make generate`.

### Database

Migrations are managed using the [goose](https://github.com/pressly/goose) utility.
