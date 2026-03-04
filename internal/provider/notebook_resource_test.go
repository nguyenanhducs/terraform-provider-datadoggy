// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// providerBlock returns the provider configuration block for acceptance tests.
func providerBlock() string {
	return `
provider "datadog" {}
`
}

// TestAccNotebookResource_basic tests the create/read/destroy lifecycle (US1).
func TestAccNotebookResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNotebookResourceConfigBasic("Test Notebook Basic"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("datadog_notebook.test", "id"),
					resource.TestCheckResourceAttr("datadog_notebook.test", "name", "Test Notebook Basic"),
					resource.TestCheckResourceAttr("datadog_notebook.test", "type", "runbook"),
				),
			},
			{
				// Verify no-op plan after apply.
				Config:   testAccNotebookResourceConfigBasic("Test Notebook Basic"),
				PlanOnly: true,
			},
		},
	})
}

func testAccNotebookResourceConfigBasic(name string) string {
	return fmt.Sprintf(`%s
resource "datadog_notebook" "test" {
  name = %q
  type = "runbook"

  cells = jsonencode([
    {
      type       = "notebook_cells"
      attributes = {
        definition = {
          type = "markdown"
          text = "## Overview"
        }
      }
    }
  ])

  time = {
    live_span = "1h"
  }
}
`, providerBlock(), name)
}

// TestAccNotebookResource_updateCells tests in-place cell updates (US2).
func TestAccNotebookResource_updateCells(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNotebookResourceConfigUpdateCells(1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("datadog_notebook.test", "id"),
				),
			},
			{
				Config: testAccNotebookResourceConfigUpdateCells(2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("datadog_notebook.test", "id"),
				),
			},
		},
	})
}

func testAccNotebookResourceConfigUpdateCells(version int) string {
	text := "## Cell 1"
	if version >= 2 {
		text = "## Cell 1 Updated"
	}
	return fmt.Sprintf(`%s
resource "datadog_notebook" "test" {
  name  = "Test Notebook Update Cells"
  cells = jsonencode([{type="notebook_cells",attributes={definition={type="markdown",text=%q}}}])
  time  = { live_span = "1h" }
}
`, providerBlock(), text)
}

// TestAccNotebookResource_withTeams tests adding/removing teams (US2).
func TestAccNotebookResource_withTeams(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNotebookResourceConfigWithTeams([]string{"team:sre"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("datadog_notebook.test", "id"),
					resource.TestCheckResourceAttr("datadog_notebook.test", "teams.#", "1"),
				),
			},
			{
				Config: testAccNotebookResourceConfigWithTeams([]string{"team:sre", "team:platform"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("datadog_notebook.test", "id"),
					resource.TestCheckResourceAttr("datadog_notebook.test", "teams.#", "2"),
				),
			},
		},
	})
}

func testAccNotebookResourceConfigWithTeams(teams []string) string {
	teamsStr := `["team:sre"]`
	if len(teams) == 2 {
		teamsStr = `["team:sre", "team:platform"]`
	}
	return fmt.Sprintf(`%s
resource "datadog_notebook" "test" {
  name  = "Test Notebook Teams"
  teams = %s
  cells = jsonencode([{"type":"notebook_cells","attributes":{"definition":{"type":"markdown","text":"## Teams test"}}}])
  time  = { live_span = "1h" }
}
`, providerBlock(), teamsStr)
}

// TestAccNotebookResource_withTemplateVars tests template variable CRUD (US2).
func TestAccNotebookResource_withTemplateVars(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNotebookResourceConfigWithTemplateVars(false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("datadog_notebook.test", "id"),
					resource.TestCheckResourceAttr("datadog_notebook.test", "template_variables.#", "1"),
				),
			},
			{
				Config: testAccNotebookResourceConfigWithTemplateVars(true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("datadog_notebook.test", "id"),
					resource.TestCheckResourceAttr("datadog_notebook.test", "template_variables.#", "2"),
				),
			},
		},
	})
}

func testAccNotebookResourceConfigWithTemplateVars(twoVars bool) string {
	tvBlock := `
  template_variables = [
    {
      name   = "host"
      prefix = "host"
    }
  ]`
	if twoVars {
		tvBlock = `
  template_variables = [
    {
      name   = "host"
      prefix = "host"
    },
    {
      name   = "env"
      prefix = "env"
    }
  ]`
	}
	return fmt.Sprintf(`%s
resource "datadog_notebook" "test" {
  name  = "Test Notebook Template Vars"
  cells = jsonencode([{"type":"notebook_cells","attributes":{"definition":{"type":"markdown","text":"## $host"}}}])
  time  = { live_span = "1h" }
%s
}
`, providerBlock(), tvBlock)
}

// TestAccNotebookResource_withLiveSpan tests changing the live_span value (US2).
func TestAccNotebookResource_withLiveSpan(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNotebookResourceConfigWithTime(`time = { live_span = "1h" }`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("datadog_notebook.test", "id"),
					resource.TestCheckResourceAttr("datadog_notebook.test", "time.live_span", "1h"),
				),
			},
			{
				Config: testAccNotebookResourceConfigWithTime(`time = { live_span = "4h" }`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("datadog_notebook.test", "id"),
					resource.TestCheckResourceAttr("datadog_notebook.test", "time.live_span", "4h"),
				),
			},
		},
	})
}

// TestAccNotebookResource_withAbsoluteTime tests switching to absolute time (US2).
func TestAccNotebookResource_withAbsoluteTime(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNotebookResourceConfigWithTime(`time = { live_span = "1h" }`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("datadog_notebook.test", "id"),
				),
			},
			{
				Config: testAccNotebookResourceConfigWithTime(`time = { start = "2024-01-01T00:00:00Z", end = "2024-01-01T06:00:00Z" }`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("datadog_notebook.test", "id"),
				),
			},
		},
	})
}

func testAccNotebookResourceConfigWithTime(timeBlock string) string {
	return fmt.Sprintf(`%s
resource "datadog_notebook" "test" {
  name  = "Test Notebook Time"
  cells = jsonencode([{"type":"notebook_cells","attributes":{"definition":{"type":"markdown","text":"## Time test"}}}])
  %s
}
`, providerBlock(), timeBlock)
}

// TestAccNotebookResource_withType tests changing the type enum (US2).
func TestAccNotebookResource_withType(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNotebookResourceConfigWithType("runbook"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("datadog_notebook.test", "id"),
					resource.TestCheckResourceAttr("datadog_notebook.test", "type", "runbook"),
				),
			},
			{
				Config: testAccNotebookResourceConfigWithType("postmortem"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("datadog_notebook.test", "id"),
					resource.TestCheckResourceAttr("datadog_notebook.test", "type", "postmortem"),
				),
			},
		},
	})
}

func testAccNotebookResourceConfigWithType(nbType string) string {
	return fmt.Sprintf(`%s
resource "datadog_notebook" "test" {
  name  = "Test Notebook Type"
  type  = %q
  cells = jsonencode([{"type":"notebook_cells","attributes":{"definition":{"type":"markdown","text":"## Type test"}}}])
  time  = { live_span = "1h" }
}
`, providerBlock(), nbType)
}

// TestAccNotebookResource_import tests import functionality (US3).
func TestAccNotebookResource_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNotebookResourceConfigBasic("Test Notebook Import"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("datadog_notebook.test", "id"),
				),
			},
			{
				ResourceName:      "datadog_notebook.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
