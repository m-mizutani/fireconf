# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`fireconf` is a Firestore configuration tool written in Go. The project is currently in its initial development stage.

## Development Commands

### Build
```bash
go build -o fireconf main.go
```

### Run
```bash
go run main.go
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
```

### Linting
```bash
go fmt ./...  # Format all Go files
go vet ./...  # Run Go vet on all packages
```

## 3rd Party Library and Tool

- Error handling: github.com/m-mizutani/goerr/v2
- Logging: log/slog
- Logging decoration: github.com/m-mizutani/clog
- Logger handling/propagation: github.com/m-mizutani/ctxlog
- Test framework: github.com/m-mizutani/gt
- CLI framework: github.com/urfave/cli/v3

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

As a Firestore configuration tool, this project will likely need to:
- Connect to Google Cloud Firestore
- Manage Firestore configurations (collections, documents, security rules)
- Handle authentication with Google Cloud Platform

When implementing features, consider:
- Using the official Firebase Admin SDK for Go or Cloud Firestore client library
- Implementing proper error handling for network and API operations
- Following Go best practices for CLI tools (consider using cobra/viper for complex CLIs)