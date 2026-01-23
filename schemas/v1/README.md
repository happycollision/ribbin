# Ribbin JSON Schemas

This directory contains the JSON Schema definitions for ribbin configuration files.

## Schema Files

### ribbin.schema.json (Loose)

The **loose schema** is intended for downstream users configuring ribbin in their projects. It:

- Validates required properties and correct types
- **Allows** additional properties not defined in the schema

This permissive approach lets users add custom metadata or comments-as-properties without validation errors, and provides forward compatibility when new versions add properties.

### ribbin.schema.strict.json (Strict)

The **strict schema** is used internally for validating ribbin's own example configs. It:

- Validates required properties and correct types
- **Disallows** additional properties (`"additionalProperties": false`)

This catches typos in property names and ensures example configs only use documented properties.

## Versioning

Both schemas are versioned together. When updating the schema:

1. Update `ribbin.schema.json` with new properties
2. Update `ribbin.schema.strict.json` with the same changes plus `"additionalProperties": false`
3. Run `make test` to verify schemas differ only in `additionalProperties`

## Build Process

The schemas are copied to `internal/config/schemas/v1/` during build for Go embedding. This happens automatically as part of `make build` and the Docker test builds. The copied directory is gitignored.

## Usage

Reference the loose schema in your `ribbin.jsonc`:

```jsonc
{
  "$schema": "https://github.com/happycollision/ribbin/schemas/v1/ribbin.schema.json",
  "wrappers": {
    // ...
  }
}
```
