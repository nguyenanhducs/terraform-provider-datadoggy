resource "datadog_notebook" "example" {
  name = "Example Service Runbook"
  type = "runbook"

  cells = jsonencode([
    {
      type = "notebook_cells"
      attributes = {
        definition = {
          type = "markdown"
          text = "## Overview\nThis runbook covers the service deploy process."
        }
      }
    }
  ])

  teams = ["team:sre"]

  time = {
    live_span = "1h"
  }
}

output "notebook_id" {
  value = datadog_notebook.example.id
}
