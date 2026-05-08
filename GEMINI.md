# GEMINI.md - kube2iam

## Project Overview
`kube2iam` provides IAM credentials to containers running inside a kubernetes cluster based on annotations.

## Build and Development

### Prerequisites
- Go 1.26.0+
- Docker (for multi-arch builds)
- Helm (for chart management)

### Core Commands
- **Install tools:** `make setup`
- **Build (local):** `make build`
- **Lint:** `make check`
- **Unit & Integration Tests:** `make test-all`
- **E2E Tests:** `make test-e2e` (requires Docker and Kind)

## CI/CD Pipeline
The project uses GitHub Actions (`.github/workflows/ci-cd.yaml`) for:
- Linting and Unit/Integration tests on every PR and push.
- E2E tests on Kind.
- Docker image releases on tags.
- Helm chart releases (OCI) on tags.
- **GitHub Draft Releases:** Automated on tags, depends on all other release jobs.

## Coding Patterns
- **AWS SDK:** Uses AWS SDK for Go v2.
- **Kubernetes:** Uses `client-go` and `e2e-framework` for testing.
- **Iptables:** Uses `go-iptables` for metadata redirection.
- **Logging:** Uses `logrus` for structured logging.

## Testing Standards
- **Unit tests:** Use `go test ./...`.
- **Integration tests:** Use `go test -tags=integration ./...`.
- **Coverage:** Minimum threshold is 50%. Run `make test-all` to verify.
