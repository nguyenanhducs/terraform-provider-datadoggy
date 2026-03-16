package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"datadoggo": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck verifies that the required Datadog credentials are set.
func testAccPreCheck(t *testing.T) {
	t.Helper()
	if v := os.Getenv("DD_API_KEY"); v == "" {
		t.Fatal("DD_API_KEY must be set for acceptance tests")
	}
	if v := os.Getenv("DD_APP_KEY"); v == "" {
		t.Fatal("DD_APP_KEY must be set for acceptance tests")
	}
}
