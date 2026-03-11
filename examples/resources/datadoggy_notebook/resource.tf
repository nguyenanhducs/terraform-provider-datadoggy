resource "datadoggy_notebook" "example" {
  name = "Example Service Runbook"
  type = "runbook"
  template_variables = [
    {
      name    = "service"
      prefix  = "service"
      default = "*"
    },
    {
      name             = "zone_name"
      prefix           = "zone_name"
      default          = "optimizely.com"
      available_values = ["optimizely.com"]
    }
  ]

  cells = jsonencode([
    {
      type = "notebook_cells"
      attributes = {
        definition = {
          type = "iframe"
          url  = "https://www.optimizely.com"
        }
        graph_size = "xl"
      }
    },
    {
      type = "notebook_cells"
      attributes = {
        definition = {
          type = "markdown"
          text = "# Optimizely: World's leading AI-powered digital experiences\n\n[Learn more about Optimizely](https://www.optimizely.com)"
        }
      }
    },
    {
      type = "notebook_cells"
      attributes = {
        definition = {
          type = "markdown"
          text = <<-EOT
            ## AI-powered digital experiences that convert quickly

            Empowering marketing and digital teams with everything they need to create content fast, test and personalize with ease, and prove impact on business.
          EOT
        }
      }
    }
  ])

  teams = ["sre"]

  time = {
    live_span = "1h"
  }
}

output "notebook_id" {
  value = datadoggy_notebook.example.id
}
