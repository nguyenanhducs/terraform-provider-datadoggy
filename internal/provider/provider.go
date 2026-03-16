package provider

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	datadogV1 "github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure DatadogProvider satisfies the provider interface.
var _ provider.Provider = &DatadogProvider{}

// DatadogProvider defines the provider implementation.
type DatadogProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// DatadogProviderModel describes the provider data model.
type DatadogProviderModel struct {
	APIKey   types.String `tfsdk:"api_key"`
	AppKey   types.String `tfsdk:"app_key"`
	APIURL   types.String `tfsdk:"api_url"`
	Validate types.Bool   `tfsdk:"validate"`
}

// DatadogClients holds the authenticated Datadog API clients passed to resources.
type DatadogClients struct {
	NotebooksAPI *datadogV1.NotebooksApi
	APIContext   context.Context
}

// Metadata returns the provider type name and version.
func (p *DatadogProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "datadoggo"
	resp.Version = p.version
}

// Schema returns the provider configuration schema.
func (p *DatadogProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Terraform provider for managing Datadog Notebook.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "Datadog API key. Can also be set via `DD_API_KEY` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"app_key": schema.StringAttribute{
				MarkdownDescription: "Datadog Application key. Can also be set via `DD_APP_KEY` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"api_url": schema.StringAttribute{
				MarkdownDescription: "Datadog API base URL. Can also be set via `DD_HOST` environment variable. Defaults to `https://api.datadoghq.com`.",
				Optional:            true,
			},
			"validate": schema.BoolAttribute{
				MarkdownDescription: "Whether to validate credentials on provider initialization. Defaults to `true`.",
				Optional:            true,
			},
		},
	}
}

// Configure sets up the Datadog API clients for use by resources.
func (p *DatadogProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data DatadogProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve api_key with env var fallback.
	apiKey := data.APIKey.ValueString()
	if apiKey == "" {
		apiKey = os.Getenv("DD_API_KEY")
	}

	// Resolve app_key with env var fallback.
	appKey := data.AppKey.ValueString()
	if appKey == "" {
		appKey = os.Getenv("DD_APP_KEY")
	}

	// Resolve api_url with env var fallback.
	apiURL := data.APIURL.ValueString()
	if apiURL == "" {
		apiURL = os.Getenv("DD_HOST")
	}
	if apiURL == "" {
		apiURL = "https://api.datadoghq.com"
	}

	// Prepend https:// only when no scheme is present (e.g. DD_HOST="us3.datadoghq.com").
	// If a scheme other than http/https is present, the validation below will reject it.
	if !strings.Contains(apiURL, "://") {
		apiURL = "https://api." + apiURL
	}

	// Validate api_url structure: parse the URL, require http/https scheme and non-empty host.
	// This catches typos like "ftp://..." or "https://" before they produce cryptic API errors.
	parsedURL, parseErr := url.Parse(apiURL)
	if parseErr != nil {
		resp.Diagnostics.AddError(
			"Invalid api_url",
			fmt.Sprintf("api_url could not be parsed as a URL: %s", parseErr),
		)
		return
	}
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		resp.Diagnostics.AddError(
			"Invalid api_url",
			fmt.Sprintf("api_url must use http or https scheme; got %q", parsedURL.Scheme),
		)
		return
	}
	if parsedURL.Host == "" {
		resp.Diagnostics.AddError(
			"Invalid api_url",
			"api_url must include a non-empty host (e.g. https://api.datadoghq.com)",
		)
		return
	}

	// Validate api_url does not end with /api/ before stripping the trailing slash,
	// since TrimRight would remove the slash and defeat the check.
	if strings.HasSuffix(apiURL, "/api/") || strings.HasSuffix(apiURL, "/api") {
		resp.Diagnostics.AddError(
			"Invalid api_url",
			"api_url must not end with /api/; use https://api.datadoghq.com instead",
		)
		return
	}

	// Strip trailing slash so the SDK path does not produce double slashes.
	apiURL = strings.TrimRight(apiURL, "/")

	// Build the Datadog SDK configuration.
	configuration := datadog.NewConfiguration()
	configuration.Servers = datadog.ServerConfigurations{
		{URL: apiURL},
	}
	apiClient := datadog.NewAPIClient(configuration)

	// Inject auth keys into a background context so it outlives the Configure call.
	authCtx := context.WithValue(
		context.Background(),
		datadog.ContextAPIKeys,
		map[string]datadog.APIKey{
			"apiKeyAuth": {Key: apiKey},
			"appKeyAuth": {Key: appKey},
		},
	)

	clients := &DatadogClients{
		NotebooksAPI: datadogV1.NewNotebooksApi(apiClient),
		APIContext:   authCtx,
	}

	resp.DataSourceData = clients
	resp.ResourceData = clients
}

// Resources returns the list of resources implemented by this provider.
func (p *DatadogProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewNotebookResource,
	}
}

// DataSources returns the list of data sources implemented by this provider.
func (p *DatadogProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

// New returns a function that creates a new provider instance.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &DatadogProvider{
			version: version,
		}
	}
}
