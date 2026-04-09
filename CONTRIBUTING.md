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

## Testing the provider locally with Terraform

To exercise a local build of the provider from a real Terraform configuration:

1. Install the provider binary locally:

```shell
make install
```

`make install` runs `go install`, which places `terraform-provider-datadoggo` in `$(go env GOBIN)` or, if `GOBIN` is unset, `$(go env GOPATH)/bin`.

2. Point Terraform at that local binary with your Terraform CLI config at `~/.terraformrc`:

```hcl
# ~/.terraformrc
provider_installation {
  dev_overrides {
    "nguyenanhducs/datadoggo" = "/path/to/your/go/bin"
  }

  direct {}
}
```

For example, if `GOBIN` is not set, `/path/to/your/go/bin` is usually `$(go env GOPATH)/bin`.

## Regenerating docs

```shell
make generate
```

## Linting

```shell
make lint
```
