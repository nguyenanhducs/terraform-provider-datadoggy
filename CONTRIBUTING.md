# Contributing

Thank you for your interest in contributing to the Datadog Notebook Terraform provider! We welcome contributions of all kinds, including bug fixes, new features, documentation improvements, and more. To ensure a smooth contribution process, please follow the guidelines outlined in this document.

## Requirements

- [Go](https://golang.org/doc/install) >= 1.24
- [golangci-lint](https://golangci-lint.run/usage/install/)
- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0 (for doc generation)

## Building from source

```shell
git clone https://github.com/nguyenanhducs/terraform-provider-datadog-notebook
cd terraform-provider-datadog-notebook
make build
```

## Running tests

```shell
# Unit tests
make test

# Acceptance tests — create real resources in Datadog, requires credentials
export DD_API_KEY="..."
export DD_APP_KEY="..."
export DD_HOST="us3.datadoghq.com"   # optional, defaults to datadoghq.com
make testacc
```

## Regenerating docs

```shell
make generate
```

## Linting

```shell
make lint
```
