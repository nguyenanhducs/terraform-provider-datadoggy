# Datadog Notebook Terraform Provider

The official `DataDog/datadog` Terraform provider exposes most Datadog resources but omits the Notebooks. This provider implements the `datadog_notebook` resource so notebooks can be managed as infrastructure-as-code alongside the rest of your Datadog setup.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- Datadog API key and Application key

## Usage

### Provider configuration

```hcl
terraform {
  required_providers {
    datadog-notebook = {
      source  = "nguyenanhducs/datadog-notebook"
      version = "~> 0.1"
    }
  }
}

provider "datadog-notebook" {
  # Credentials can be omitted if DD_API_KEY and DD_APP_KEY env vars are set.
  api_key = var.datadog_api_key
  app_key = var.datadog_app_key
}
```

### Creating a notebook

```hcl
resource "datadog_notebook" "service_runbook" {
  name = "My Service Runbook"
  type = "runbook"

  cells = jsonencode([
    {
      type = "notebook_cells"
      attributes = {
        definition = {
          type = "markdown"
          text = "## Overview\nDescribe your service here."
        }
      }
    }
  ])

  time = {
    live_span = "1h"
  }
}

output "notebook_id" {
  value = datadog_notebook.service_runbook.id
}
```

### Importing an existing notebook

```shell
terraform import datadog_notebook.example <notebook_id>
```

## Environment variables

| Variable     | Description                                           |
|--------------|-------------------------------------------------------|
| `DD_API_KEY` | Datadog API key (alternative to `api_key` in config) |
| `DD_APP_KEY` | Datadog Application key (alternative to `app_key`)   |
| `DD_HOST`    | Datadog API base URL, e.g. `us3.datadoghq.com`       |

## Resources

| Resource           | Description                                    |
|--------------------|------------------------------------------------|
| `datadog_notebook` | Manages a Datadog Notebook (CRUD + import)     |

See [`docs/resources/notebook.md`](docs/resources/notebook.md) for the full schema reference.
