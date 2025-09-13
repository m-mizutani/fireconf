# fireconf

Firestore index and TTL configuration management tool and Go library

## Overview

`fireconf` provides both a command-line tool and a Go library for managing Firestore composite indexes and TTL (Time-to-Live) policies. It offers a declarative way to maintain your Firestore database configuration as code, enabling version control, automated deployments, and consistent configuration across environments.

## Features

- **Go Library**: Programmatic API for Firestore configuration management
- **CLI Tool**: Command-line interface for configuration operations
- **Declarative Configuration**: Define indexes and TTL policies in YAML or Go code
- **Sync Command**: Apply configuration changes to Firestore
- **Import Command**: Export existing Firestore configuration to YAML
- **Dry Run Mode**: Preview changes before applying them
- **Idempotent Operations**: Safe to run multiple times
- **Migration Planning**: Get detailed migration plans before execution

## Installation

### CLI Tool

```bash
go install github.com/m-mizutani/fireconf/cmd/fireconf@latest
```

### Go Library

```bash
go get github.com/m-mizutani/fireconf@latest
```

## Library Usage

### Basic Example

```go
package main

import (
    "context"
    "log"

    "github.com/m-mizutani/fireconf"
)

func main() {
    ctx := context.Background()

    // Create a new fireconf client
    client, err := fireconf.NewClient(ctx, "my-project")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Define configuration programmatically
    config := &fireconf.Config{
        Collections: []fireconf.Collection{
            {
                Name: "users",
                Indexes: []fireconf.Index{
                    {
                        Fields: []fireconf.IndexField{
                            {Path: "email", Order: fireconf.OrderAscending},
                            {Path: "createdAt", Order: fireconf.OrderDescending},
                        },
                    },
                },
                TTL: &fireconf.TTL{
                    Field: "expireAt",
                },
            },
        },
    }

    // Apply configuration to Firestore
    if err := client.Migrate(ctx, config); err != nil {
        log.Fatal(err)
    }
}
```

### Loading from YAML

```go
// Load configuration from YAML file
config, err := fireconf.LoadConfigFromYAML("fireconf.yaml")
if err != nil {
    log.Fatal(err)
}

// Apply to Firestore
if err := client.Migrate(ctx, config); err != nil {
    log.Fatal(err)
}
```

### Importing Existing Configuration

```go
// Import current configuration from Firestore
config, err := client.Import(ctx, "users", "posts")
if err != nil {
    log.Fatal(err)
}

// Save to YAML
if err := config.SaveToYAML("fireconf.yaml"); err != nil {
    log.Fatal(err)
}
```

### Advanced Options

```go
client, err := fireconf.NewClient(ctx, "my-project",
    fireconf.WithLogger(logger),
    fireconf.WithDatabaseID("custom-db"),
    fireconf.WithCredentialsFile("service-account.json"),
    fireconf.WithDryRun(true),
)
```

### Migration Planning

```go
// Get migration plan without executing
plan, err := client.GetMigrationPlan(ctx, config)
if err != nil {
    log.Fatal(err)
}

for _, step := range plan.Steps {
    fmt.Printf("Step: %s - %s (destructive: %v)\n", 
        step.Operation, step.Description, step.Destructive)
}
```

## CLI Usage

### Sync Configuration

Apply index and TTL configuration from a YAML file to Firestore:

```bash
fireconf sync --project YOUR_PROJECT_ID --config fireconf.yaml
```

Options:
- `--project`, `-p`: Google Cloud project ID (required)
- `--database`, `-d`: Firestore database ID (default: "(default)")
- `--config`, `-c`: Configuration file path (default: "fireconf.yaml")
- `--dry-run`: Show what would be changed without making actual changes
- `--verbose`, `-v`: Enable verbose logging

### Import Configuration

Export existing Firestore configuration to YAML:

```bash
# Import all collections from default database
fireconf import --project YOUR_PROJECT_ID > fireconf.yaml

# Import specific collections
fireconf import --project YOUR_PROJECT_ID --collections users --collections posts --collections comments > fireconf.yaml

# Or use the short form
fireconf import --project YOUR_PROJECT_ID -c users -c posts -c comments > fireconf.yaml

# Export to a specific file
fireconf import --project YOUR_PROJECT_ID --output fireconf.yaml --collections users --collections posts

# For non-default databases, collection names must be specified explicitly
fireconf import --project YOUR_PROJECT_ID --database warren-v1 --collections users --collections posts > fireconf.yaml
```

### Validate Configuration

Validate a configuration file without applying changes:

```bash
fireconf validate --config fireconf.yaml
```

**Note**: Automatic collection discovery is only available for the default database. For non-default databases, you must specify collection names explicitly due to Firestore client limitations.

## Configuration Format

```yaml
collections:
  - name: users
    indexes:
      - fields:
          - path: email
            order: ASCENDING
          - path: createdAt
            order: DESCENDING
        queryScope: COLLECTION
      - fields:
          - path: status
            order: ASCENDING
          - path: isActive
            order: ASCENDING
        queryScope: COLLECTION_GROUP
    ttl:
      field: expireAt

  - name: posts
    indexes:
      - fields:
          - path: authorId
            order: ASCENDING
          - path: publishedAt
            order: DESCENDING
        queryScope: COLLECTION
      - fields:
          - path: tags
            arrayConfig: CONTAINS
          - path: score
            order: DESCENDING
        queryScope: COLLECTION
      - fields:
          - path: embedding
            vectorConfig:
              dimension: 768
        queryScope: COLLECTION
```

### Field Types

- **Ordered fields**: `order: ASCENDING` or `order: DESCENDING`
- **Array fields**: `arrayConfig: CONTAINS`
- **Vector fields**: `vectorConfig: { dimension: 768 }` (for vector search)

### Query Scopes

- `COLLECTION`: Index applies to a specific collection
- `COLLECTION_GROUP`: Index applies to all collections with the same ID

## Examples

See the [examples](examples/) directory for complete usage examples:

- [Basic Usage](examples/basic/main.go): Simple configuration and migration
- [Advanced Usage](examples/advanced/main.go): Using all features including vector search
- [Migration from YAML](examples/migration/main.go): Migrating existing configurations

## Authentication

The tool uses Application Default Credentials (ADC). You can authenticate using:

1. **Google Cloud environment**: Automatically uses attached service account
2. **Service account key**: Set `GOOGLE_APPLICATION_CREDENTIALS` environment variable
3. **gcloud CLI**: Run `gcloud auth application-default login`

## Required Permissions

The service account needs the following IAM permissions:

- `datastore.indexes.create`
- `datastore.indexes.delete`
- `datastore.indexes.get`
- `datastore.indexes.list`
- `datastore.indexes.update`
- `datastore.operations.list`
- `datastore.operations.get`

## API Reference

For detailed API documentation, see the [Go package documentation](https://pkg.go.dev/github.com/m-mizutani/fireconf).

## Notes

- Index creation/deletion can take several minutes to complete
- TTL policies are limited to one field per collection
- TTL field indexing is automatically disabled to prevent hotspots
- Firestore Admin API operations bypass Firestore Security Rules

## License

Apache License 2.0