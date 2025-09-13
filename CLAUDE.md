# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`fireconf` is a Firestore index and TTL configuration management tool and Go library. It allows you to manage Firestore composite indexes and TTL policies as code using YAML configuration files or programmatic Go API.

The project provides both:
- **Go Library**: Programmatic API for Firestore configuration management
- **CLI Tool**: Command-line interface for configuration operations

## Development Commands

### Build

#### CLI Tool
```bash
# Build CLI binary
go build -o fireconf cmd/fireconf/main.go
# or using Task
task build
```

#### Library
```bash
# Test library build
go build .
```

### Run

#### CLI Commands
```bash
# Using go run
go run cmd/fireconf/main.go sync --project YOUR_PROJECT --config fireconf.yaml
go run cmd/fireconf/main.go import --project YOUR_PROJECT --collections users --collections posts
go run cmd/fireconf/main.go validate --config fireconf.yaml

# or using Task
task run:sync PROJECT=my-project CONFIG=fireconf.yaml
task run:import PROJECT=my-project COLLECTIONS="users posts"
```

#### Library Examples
```bash
# Run examples
cd examples/basic && go run main.go
cd examples/advanced && go run main.go
cd examples/migration && go run main.go
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
- YAML processing: github.com/goccy/go-yaml
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
- Internal packages (`internal/`) are not accessible from outside the module

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
├── README.md
├── doc.go              # Package documentation
├── fireconf.go         # Main library API
├── config.go           # Configuration structures
├── migrate.go          # Migration functionality
├── options.go          # Client options
├── errors.go           # Custom error types
│
├── cmd/fireconf/       # CLI application
│   ├── main.go
│   └── commands/
│       ├── common.go
│       ├── sync.go
│       ├── import.go
│       └── validate.go
│
├── internal/           # Internal implementation (not accessible externally)
│   ├── adapter/        # Firestore Admin API implementation
│   │   └── firestore/
│   ├── usecase/        # Business logic
│   ├── model/          # Internal domain models
│   └── interfaces/     # Internal interfaces and mocks
│       └── mock/
│
├── examples/           # Usage examples
│   ├── basic/
│   ├── advanced/
│   └── migration/
│
└── testdata/          # Test data
```

The project follows a clean architecture pattern with:
- **Root**: Public library API
- **cmd/fireconf**: CLI application entry point
- **internal/**: Private implementation details
  - `adapter/`: Firestore Admin API implementation
  - `usecase/`: Application business logic
  - `model/`: Internal domain models
  - `interfaces/`: Internal interfaces

## Architecture Notes

The project follows clean architecture principles:

### Package Structure
- **Root package**: Public API for library users
- `cmd/fireconf`: CLI application that uses the public library API
- `internal/model`: Internal domain models for configuration (YAML mapping)
- `internal/interfaces`: Interfaces for Firestore operations (with mocks)
- `internal/adapter/firestore`: Firestore Admin API implementation
- `internal/usecase`: Business logic for sync, import, and validation operations

### Key Design Decisions
1. **Library + CLI Architecture**: Root package provides Go library API, CLI is built on top of it
2. **Firestore Admin API**: Uses `cloud.google.com/go/firestore/apiv1/admin` instead of regular Firestore SDK for index/TTL management
3. **Idempotent Operations**: All operations are designed to be safely run multiple times
4. **TTL Field Indexing**: Automatically disables single-field indexes on TTL fields to prevent hotspots
5. **Error Handling**: Uses `github.com/m-mizutani/goerr/v2` for structured error handling with context
6. **Internal/External Separation**: Public API is separate from internal implementation

### Authentication
- Supports Application Default Credentials (ADC)
- Can use service account key files via environment variable
- Requires specific IAM permissions for datastore index and operation management

### Type Conversion
- Public API uses clean, Go-idiomatic types
- Internal models handle YAML marshaling/unmarshaling
- Conversion layer between public API and internal models

# important-instruction-reminders
Do what has been asked; nothing more, nothing less.
NEVER create files unless they're absolutely necessary for achieving your goal.
ALWAYS prefer editing an existing file to creating a new one.
NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested by the User.