package provider

import (
	"context"
	"testing"

	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestDatadogProviderMetadata(t *testing.T) {
	p := &DatadogProvider{version: "test"}
	req := fwprovider.MetadataRequest{}
	resp := &fwprovider.MetadataResponse{}
	p.Metadata(context.Background(), req, resp)
	if resp.TypeName != "datadoggo" {
		t.Errorf("expected TypeName 'datadoggo', got %q", resp.TypeName)
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

// ---------------------------------------------------------------------------
// Configure — api_url validation (T025, T026, T027)
// ---------------------------------------------------------------------------

// makeProviderConfigureRequest constructs a provider.ConfigureRequest with the
// given api_url value (empty string means null/unset) for Configure() unit tests.
func makeProviderConfigureRequest(t *testing.T, apiURL string) fwprovider.ConfigureRequest {
	t.Helper()
	p := &DatadogProvider{}
	schemaResp := &fwprovider.SchemaResponse{}
	p.Schema(context.Background(), fwprovider.SchemaRequest{}, schemaResp)

	apiURLVal := tftypes.NewValue(tftypes.String, nil) // null
	if apiURL != "" {
		apiURLVal = tftypes.NewValue(tftypes.String, apiURL)
	}
	configVal := tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"api_key":  tftypes.String,
				"app_key":  tftypes.String,
				"api_url":  tftypes.String,
				"validate": tftypes.Bool,
			},
		},
		map[string]tftypes.Value{
			"api_key":  tftypes.NewValue(tftypes.String, nil),
			"app_key":  tftypes.NewValue(tftypes.String, nil),
			"api_url":  apiURLVal,
			"validate": tftypes.NewValue(tftypes.Bool, nil),
		},
	)
	return fwprovider.ConfigureRequest{
		Config: tfsdk.Config{
			Raw:    configVal,
			Schema: schemaResp.Schema,
		},
	}
}

// TestConfigureInvalidAPIURLScheme verifies that Configure rejects api_url values
// that use a scheme other than http or https.
func TestConfigureInvalidAPIURLScheme(t *testing.T) {
	p := &DatadogProvider{}
	req := makeProviderConfigureRequest(t, "ftp://api.datadoghq.com")
	resp := &fwprovider.ConfigureResponse{}
	p.Configure(context.Background(), req, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error diagnostic for ftp:// scheme, got none")
	}
}

// TestConfigureInvalidAPIURLNoHost verifies that Configure rejects api_url values
// that have a valid scheme but an empty host.
func TestConfigureInvalidAPIURLNoHost(t *testing.T) {
	p := &DatadogProvider{}
	req := makeProviderConfigureRequest(t, "https://")
	resp := &fwprovider.ConfigureResponse{}
	p.Configure(context.Background(), req, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error diagnostic for empty host, got none")
	}
}

// TestConfigureInvalidAPIURLAPIPath verifies that Configure still rejects api_url
// values ending with /api/ (the original validation remains in place).
func TestConfigureInvalidAPIURLAPIPath(t *testing.T) {
	p := &DatadogProvider{}
	req := makeProviderConfigureRequest(t, "https://api.datadoghq.com/api/")
	resp := &fwprovider.ConfigureResponse{}
	p.Configure(context.Background(), req, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error diagnostic for /api/ suffix, got none")
	}
}
