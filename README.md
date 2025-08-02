# fireconf

Firestore index and TTL configuration management tool

## Overview

`fireconf` is a command-line tool that manages Firestore composite indexes and TTL (Time-to-Live) policies using YAML configuration files. It provides a declarative way to maintain your Firestore database configuration as code.

## Features

- **Declarative Configuration**: Define indexes and TTL policies in YAML
- **Sync Command**: Apply configuration changes to Firestore
- **Import Command**: Export existing Firestore configuration to YAML
- **Dry Run Mode**: Preview changes before applying them
- **Idempotent Operations**: Safe to run multiple times

## Installation

```bash
go install github.com/m-mizutani/fireconf@latest
```

## Usage

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
fireconf import --project YOUR_PROJECT_ID users posts comments > fireconf.yaml

# Export to a specific file
fireconf import --project YOUR_PROJECT_ID --output fireconf.yaml users posts

# For non-default databases, collection names must be specified explicitly
fireconf import --project YOUR_PROJECT_ID --database warren-v1 users posts > fireconf.yaml
```

**Note**: Automatic collection discovery is only available for the default database. For non-default databases, you must specify collection names explicitly due to Firestore client limitations.

## Configuration Format

```yaml
collections:
  - name: users
    indexes:
      - fields:
          - name: email
            order: ASCENDING
          - name: createdAt
            order: DESCENDING
        queryScope: COLLECTION
      - fields:
          - name: status
            order: ASCENDING
          - name: isActive
            order: ASCENDING
        queryScope: COLLECTION_GROUP
    ttl:
      field: expireAt

  - name: posts
    indexes:
      - fields:
          - name: authorId
            order: ASCENDING
          - name: publishedAt
            order: DESCENDING
        queryScope: COLLECTION
      - fields:
          - name: tags
            arrayConfig: CONTAINS
          - name: score
            order: DESCENDING
        queryScope: COLLECTION
```

### Field Types

- **Ordered fields**: `order: ASCENDING` or `order: DESCENDING`
- **Array fields**: `arrayConfig: CONTAINS`
- **Vector fields**: `vectorConfig: { dimension: 768 }` (experimental)

### Query Scopes

- `COLLECTION`: Index applies to a specific collection
- `COLLECTION_GROUP`: Index applies to all collections with the same ID

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

## Notes

- Index creation/deletion can take several minutes to complete
- TTL policies are limited to one field per collection
- TTL field indexing is automatically disabled to prevent hotspots
- Firestore Admin API operations bypass Firestore Security Rules

## License

Apache License 2.0
