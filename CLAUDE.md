# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`fireconf` is a Firestore index and TTL configuration management tool. It allows you to manage Firestore composite indexes and TTL policies as code using YAML configuration files.

## Development Commands

### Build
```bash
go build -o fireconf main.go
# or using Task
task build
```

### Run
```bash
go run main.go sync --project YOUR_PROJECT --config fireconf.yaml
go run main.go import --project YOUR_PROJECT users posts

# or using Task
task run:sync PROJECT=my-project CONFIG=fireconf.yaml
task run:import PROJECT=my-project COLLECTIONS="users posts"
```

### Module Management
```bash
go mod tidy  # Add missing and remove unused modules
go mod download  # Download modules to local cache
```

### Testing
```bash
go test ./...  # Run all tests
go test -v ./...  # Run all tests with verbose output
go test -cover ./...  # Run tests with coverage

# or using Task
task test
task test:cover
```

### Linting and Checks
```bash
go fmt ./...  # Format all Go files
go vet ./...  # Run Go vet on all packages
golangci-lint run ./...  # Run golangci-lint
gosec -quiet ./...  # Run security scan

# or using Task
task check  # Run all checks
```

### Mock Generation
```bash
go generate ./...  # Generate all mocks
# or
task generate
task mock  # Generate FirestoreClient mock specifically
```

## 3rd Party Library and Tool

- Error handling: github.com/m-mizutani/goerr/v2
- Logging: log/slog
- Logging decoration: github.com/m-mizutani/clog
- Logger handling/propagation: github.com/m-mizutani/ctxlog
- Test framework: github.com/m-mizutani/gt
- CLI framework: github.com/urfave/cli/v3
- Mock generation tool: github.com/matryer/moq
- Task runner: https://github.com/go-task/task

## Restriction & Rule

### Directory

- When you are mentioned about `tmp` directory, you SHOULD NOT see `/tmp`. You need to check `./tmp` directory from root of the repository.
- When you need to build or create some temporary file(s), you should use `./tmp` directory under root of the project.

### Exposure policy

In principle, do not trust developers who use this library from outside

- Do not export unnecessary methods, structs, and variables
- Assume that exposed items will be changed. Never expose fields that would be problematic if changed
- Use `export_test.go` for items that need to be exposed for testing purposes

### Check

When making changes, before finishing the task, always:
- Run `go vet ./...`, `go fmt ./...` to format the code
- Run `golangci-lint run ./...` to check lint error
- Run `gosec -quiet ./...` to check security issue
- Run tests to ensure no impact on other code

### Language

All comment and character literal in source code must be in English

### Testing

- Test files should have `package {name}_test`. Do not use same package name
- **🚨 CRITICAL RULE: Test MUST be included in same name test file. (e.g. test for `abc.go` must be in `abc_test.go`) 🚨**
  - **NEVER create test files like:**
    - ❌ `e2e_test.go`
    - ❌ `integration_test.go`
    - ❌ `feature_xyz_test.go`
    - ❌ `log_test.go` (unless there's a `log.go`)
  - **ALWAYS match the source file name:**
    - ✅ `server.go` → `server_test.go`
    - ✅ `middleware.go` → `middleware_test.go`
    - ✅ `alert.go` → `alert_test.go`
  - **Before creating ANY test, ask: "Which source file does this test belong to?"**
  - **If testing multiple files' interaction, put the test in the primary file's test**
- Do not build binary. If you need to run, use `go run` command instead
- Extend timeout duration if the test fails with time out
- DO NOT use `-short`

### Test File Checklist (Use this EVERY time)
Before creating or modifying tests:
1. ✓ Is there a corresponding source file for this test file?
2. ✓ Does the test file name match exactly? (`xyz.go` → `xyz_test.go`)
3. ✓ Are all tests for a source file in ONE test file?
4. ✓ No standalone feature/e2e/integration test files?


## Project Structure

```
.
├── .gitignore
├── CLAUDE.md
├── go.mod
├── LICENSE
├── main.go
├── pkg/
│   ├── cli/          # CLI interface and commands
│   ├── domain/       # Core domain logic
│   │   ├── interfaces/
│   │   └── model/
│   └── usecase/      # Application use cases
└── README.md
```

The project follows a clean architecture pattern with:
- `main.go`: Entry point for the application
- `pkg/`: Main source code directory
  - `cli/`: Command-line interface implementation
  - `domain/`: Core business logic and domain models
  - `usecase/`: Application-specific business rules

## Architecture Notes

The project follows clean architecture principles:

### Package Structure
- `pkg/domain/model`: Domain models for configuration (YAML)
- `pkg/domain/interfaces`: Interfaces for Firestore operations
- `pkg/adapter/firestore`: Firestore Admin API implementation
- `pkg/usecase`: Business logic for sync and import operations
- `pkg/cli`: CLI command implementations using urfave/cli/v3

### Key Design Decisions
1. **Firestore Admin API**: Uses `cloud.google.com/go/firestore/apiv1/admin` instead of regular Firestore SDK for index/TTL management
2. **Idempotent Operations**: All operations are designed to be safely run multiple times
3. **TTL Field Indexing**: Automatically disables single-field indexes on TTL fields to prevent hotspots
4. **Error Handling**: Uses `github.com/m-mizutani/goerr/v2` for structured error handling with context

### Authentication
- Supports Application Default Credentials (ADC)
- Can use service account key files via environment variable
- Requires specific IAM permissions for datastore index and operation management