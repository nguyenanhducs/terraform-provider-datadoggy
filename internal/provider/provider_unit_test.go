// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
)

func TestDatadogProviderMetadata(t *testing.T) {
	p := &DatadogProvider{version: "test"}
	req := fwprovider.MetadataRequest{}
	resp := &fwprovider.MetadataResponse{}
	p.Metadata(context.Background(), req, resp)
	if resp.TypeName != "datadog" {
		t.Errorf("expected TypeName 'datadog', got %q", resp.TypeName)
	}
	if resp.Version != "test" {
		t.Errorf("expected Version 'test', got %q", resp.Version)
	}
}

func TestDatadogProviderSchema(t *testing.T) {
	p := &DatadogProvider{}
	req := fwprovider.SchemaRequest{}
	resp := &fwprovider.SchemaResponse{}
	p.Schema(context.Background(), req, resp)

	attrs := resp.Schema.Attributes
	for _, name := range []string{"api_key", "app_key", "api_url", "validate"} {
		if _, ok := attrs[name]; !ok {
			t.Errorf("expected attribute %q in provider schema", name)
		}
	}
}

func TestDatadogProviderResources(t *testing.T) {
	p := &DatadogProvider{}
	resources := p.Resources(context.Background())
	if len(resources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(resources))
	}
}

func TestDatadogProviderDataSources(t *testing.T) {
	p := &DatadogProvider{}
	dataSources := p.DataSources(context.Background())
	if len(dataSources) != 0 {
		t.Errorf("expected 0 data sources, got %d", len(dataSources))
	}
}
