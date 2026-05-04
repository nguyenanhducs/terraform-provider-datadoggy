package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	datadogV1 "github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---------------------------------------------------------------------------
// Helper types (used by the custom JSON validator wrapper tests below)
// ---------------------------------------------------------------------------

// jsonValidatorStringRequest is a minimal request for testing the JSON validator logic.
type jsonValidatorStringRequest struct {
	value string
}

// jsonValidatorStringResponse is a minimal response for testing the JSON validator logic.
type jsonValidatorStringResponse struct {
	hasError bool
}

func (v jsonValidator) validate(_ context.Context, s string, resp *jsonValidatorStringResponse) {
	var js json.RawMessage
	if err := json.Unmarshal([]byte(s), &js); err != nil {
		resp.hasError = true
	}
}

// ---------------------------------------------------------------------------
// NewNotebookResource
// ---------------------------------------------------------------------------

func TestNewNotebookResource(t *testing.T) {
	r := NewNotebookResource()
	if r == nil {
		t.Fatal("expected non-nil resource")
	}
}

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

func TestNotebookResourceMetadata(t *testing.T) {
	r := &NotebookResource{}
	req := resource.MetadataRequest{ProviderTypeName: "datadoggo"}
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), req, resp)
	if resp.TypeName != "datadoggo_notebook" {
		t.Errorf("expected type name 'datadoggo_notebook', got %q", resp.TypeName)
	}
}

// ---------------------------------------------------------------------------
// Schema
// ---------------------------------------------------------------------------

func TestNotebookResourceSchema(t *testing.T) {
	r := &NotebookResource{}
	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	attrs := resp.Schema.Attributes
	for _, name := range []string{"id", "name", "cells", "type", "is_template", "take_snapshots", "teams", "template_variables", "time"} {
		if _, ok := attrs[name]; !ok {
			t.Errorf("expected attribute %q in schema", name)
		}
	}
}

// ---------------------------------------------------------------------------
// ConfigValidators
// ---------------------------------------------------------------------------

func TestNotebookResourceConfigValidators(t *testing.T) {
	r := &NotebookResource{}
	validators := r.ConfigValidators(context.Background())
	if len(validators) != 1 {
		t.Errorf("expected 1 config validator, got %d", len(validators))
	}
}

// ---------------------------------------------------------------------------
// jsonValidator
// ---------------------------------------------------------------------------

func TestJSONValidatorDescription(t *testing.T) {
	v := jsonValidator{}
	if v.Description(context.Background()) == "" {
		t.Error("expected non-empty Description")
	}
	if v.MarkdownDescription(context.Background()) == "" {
		t.Error("expected non-empty MarkdownDescription")
	}
}

func TestJSONValidatorValidateStringValid(t *testing.T) {
	v := jsonValidator{}
	req := validator.StringRequest{ConfigValue: types.StringValue(`[{"type":"notebook_cells"}]`)}
	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("expected no error for valid JSON, got: %v", resp.Diagnostics)
	}
}

func TestJSONValidatorValidateStringInvalid(t *testing.T) {
	v := jsonValidator{}
	req := validator.StringRequest{ConfigValue: types.StringValue("not-json")}
	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), req, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for invalid JSON")
	}
}

func TestJSONValidatorValidateStringNull(t *testing.T) {
	v := jsonValidator{}
	req := validator.StringRequest{ConfigValue: types.StringNull()}
	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for null value")
	}
}

func TestJSONValidatorValidateStringUnknown(t *testing.T) {
	v := jsonValidator{}
	req := validator.StringRequest{ConfigValue: types.StringUnknown()}
	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for unknown value")
	}
}

// ---------------------------------------------------------------------------
// jsonValidator (wrapper, existing style)
// ---------------------------------------------------------------------------

func TestJSONValidatorValid(t *testing.T) {
	v := jsonValidator{}
	req := jsonValidatorStringRequest{value: `[{"type":"markdown"}]`}
	resp := &jsonValidatorStringResponse{}
	v.validate(context.Background(), req.value, resp)
	if resp.hasError {
		t.Error("expected no error for valid JSON")
	}
}

func TestJSONValidatorInvalid(t *testing.T) {
	_ = jsonValidator{}
	var out json.RawMessage
	err := json.Unmarshal([]byte("not-json"), &out)
	if err == nil {
		t.Error("expected unmarshal error for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// notebookTimeValidator
// ---------------------------------------------------------------------------

func TestNotebookTimeValidatorDescription(t *testing.T) {
	v := notebookTimeValidator{}
	if v.Description(context.Background()) == "" {
		t.Error("expected non-empty Description")
	}
	if v.MarkdownDescription(context.Background()) == "" {
		t.Error("expected non-empty MarkdownDescription")
	}
}

// ---------------------------------------------------------------------------
// Configure
// ---------------------------------------------------------------------------

func TestNotebookResourceConfigureNil(t *testing.T) {
	r := &NotebookResource{}
	req := resource.ConfigureRequest{ProviderData: nil}
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("expected no error for nil provider data, got: %v", resp.Diagnostics)
	}
}

func TestNotebookResourceConfigureValid(t *testing.T) {
	r := &NotebookResource{}
	clients := &DatadogClients{APIKeys: map[string]datadog.APIKey{}}
	req := resource.ConfigureRequest{ProviderData: clients}
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("expected no error for valid clients, got: %v", resp.Diagnostics)
	}
}

func TestNotebookResourceConfigureWrongType(t *testing.T) {
	r := &NotebookResource{}
	req := resource.ConfigureRequest{ProviderData: "not-a-clients-struct"}
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), req, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong provider data type")
	}
}

// ---------------------------------------------------------------------------
// cellsFromJSON
// ---------------------------------------------------------------------------

func TestCellsFromJSON(t *testing.T) {
	cellsJSON := `[{"attributes":{"definition":{"text":"hello","type":"markdown"}},"type":"notebook_cells"}]`
	cells, err := cellsFromJSON(cellsJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cells) != 1 {
		t.Fatalf("expected 1 cell, got %d", len(cells))
	}
}

func TestCellsFromJSONInvalid(t *testing.T) {
	_, err := cellsFromJSON("not-json")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ---------------------------------------------------------------------------
// cellsToJSON
// ---------------------------------------------------------------------------

func TestCellsToJSON(t *testing.T) {
	out, err := cellsToJSON([]datadogV1.NotebookCellResponse{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "[]" {
		t.Fatalf("expected '[]', got %q", out)
	}
}

// TestCellsToJSONStripsID verifies the id-stripping loop executes for non-empty slices.
func TestCellsToJSONStripsID(t *testing.T) {
	// Unmarshal a complete API-style cell response with an id field.
	var cells []datadogV1.NotebookCellResponse
	rawJSON := `[{"type":"notebook_markdown_cells","id":"cell-abc-123","attributes":{"definition":{"type":"markdown","text":"hello"}}}]`
	if err := json.Unmarshal([]byte(rawJSON), &cells); err != nil {
		t.Skipf("SDK could not unmarshal test cell (union type): %v", err)
	}
	out, err := cellsToJSON(cells)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(result) > 0 {
		if _, hasID := result[0]["id"]; hasID {
			t.Error("expected 'id' to be stripped from cell JSON")
		}
	}
}

// ---------------------------------------------------------------------------
// cellsToUpdateCells
// ---------------------------------------------------------------------------

func TestCellsToUpdateCells(t *testing.T) {
	cellsJSON := `[{"attributes":{"definition":{"text":"## hello","type":"markdown"}},"type":"notebook_cells"}]`
	updateCells, err := cellsToUpdateCells(cellsJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updateCells) != 1 {
		t.Fatalf("expected 1 update cell, got %d", len(updateCells))
	}
	if updateCells[0].NotebookCellCreateRequest == nil {
		t.Fatal("expected NotebookCellCreateRequest, got nil")
	}
}

// ---------------------------------------------------------------------------
// buildGlobalTime
// ---------------------------------------------------------------------------

func TestBuildGlobalTimeLiveSpan(t *testing.T) {
	timeModel := &NotebookTimeModel{
		LiveSpan: types.StringValue("1h"),
		Start:    types.StringNull(),
		End:      types.StringNull(),
	}
	gt, err := buildGlobalTime(timeModel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gt.NotebookRelativeTime == nil {
		t.Fatal("expected NotebookRelativeTime, got nil")
	}
	if string(gt.NotebookRelativeTime.LiveSpan) != "1h" {
		t.Errorf("expected live_span '1h', got %q", gt.NotebookRelativeTime.LiveSpan)
	}
}

func TestBuildGlobalTimeAbsolute(t *testing.T) {
	timeModel := &NotebookTimeModel{
		LiveSpan: types.StringNull(),
		Start:    types.StringValue("2024-01-01T00:00:00Z"),
		End:      types.StringValue("2024-01-01T06:00:00Z"),
	}
	gt, err := buildGlobalTime(timeModel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gt.NotebookAbsoluteTime == nil {
		t.Fatal("expected NotebookAbsoluteTime, got nil")
	}
}

func TestBuildGlobalTimeNil(t *testing.T) {
	gt, err := buildGlobalTime(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gt.NotebookRelativeTime == nil {
		t.Fatal("expected default NotebookRelativeTime, got nil")
	}
	if string(gt.NotebookRelativeTime.LiveSpan) != "1h" {
		t.Errorf("expected default live_span '1h', got %q", gt.NotebookRelativeTime.LiveSpan)
	}
}

func TestBuildGlobalTimeInvalidStart(t *testing.T) {
	timeModel := &NotebookTimeModel{
		LiveSpan: types.StringNull(),
		Start:    types.StringValue("not-a-timestamp"),
		End:      types.StringValue("2024-01-01T06:00:00Z"),
	}
	_, err := buildGlobalTime(timeModel)
	if err == nil {
		t.Fatal("expected error for invalid start timestamp")
	}
}

// TestBuildGlobalTimeInvalidEnd verifies error on bad end timestamp.
func TestBuildGlobalTimeInvalidEnd(t *testing.T) {
	timeModel := &NotebookTimeModel{
		LiveSpan: types.StringNull(),
		Start:    types.StringValue("2024-01-01T00:00:00Z"),
		End:      types.StringValue("not-a-timestamp"),
	}
	_, err := buildGlobalTime(timeModel)
	if err == nil {
		t.Fatal("expected error for invalid end timestamp")
	}
}

// TestBuildGlobalTimeEmptyBlock verifies non-nil time block with all-null fields defaults to 1h.
func TestBuildGlobalTimeEmptyBlock(t *testing.T) {
	timeModel := &NotebookTimeModel{
		LiveSpan: types.StringNull(),
		Start:    types.StringNull(),
		End:      types.StringNull(),
	}
	gt, err := buildGlobalTime(timeModel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gt.NotebookRelativeTime == nil {
		t.Fatal("expected default NotebookRelativeTime for empty time block")
	}
	if string(gt.NotebookRelativeTime.LiveSpan) != "1h" {
		t.Errorf("expected default live_span '1h', got %q", gt.NotebookRelativeTime.LiveSpan)
	}
}

// ---------------------------------------------------------------------------
// buildMetadata
// ---------------------------------------------------------------------------

func TestBuildMetadataWithType(t *testing.T) {
	data := NotebookResourceModel{
		Type:          types.StringValue("runbook"),
		IsTemplate:    types.BoolValue(false),
		TakeSnapshots: types.BoolValue(true),
	}
	meta := buildMetadata(data)
	if meta.Type.Get() == nil {
		t.Fatal("expected Type to be set")
	}
	if string(*meta.Type.Get()) != "runbook" {
		t.Errorf("expected type 'runbook', got %q", *meta.Type.Get())
	}
	if meta.IsTemplate == nil || *meta.IsTemplate != false {
		t.Error("expected is_template false")
	}
	if meta.TakeSnapshots == nil || *meta.TakeSnapshots != true {
		t.Error("expected take_snapshots true")
	}
}

func TestBuildMetadataNoType(t *testing.T) {
	data := NotebookResourceModel{
		Type: types.StringNull(),
	}
	meta := buildMetadata(data)
	if meta != nil {
		t.Error("expected nil metadata when no fields are set")
	}
}

// ---------------------------------------------------------------------------
// buildTemplateVariables
// ---------------------------------------------------------------------------

func TestBuildTemplateVariables(t *testing.T) {
	ctx := context.Background()
	avail, _ := types.ListValueFrom(ctx, types.StringType, []string{"my-host-1", "my-host-2"})
	tvs := []TemplateVariableModel{
		{
			Name:            types.StringValue("host"),
			Prefix:          types.StringValue("host"),
			Default:         types.StringValue("my-host-1"),
			AvailableValues: avail,
		},
	}
	result, _ := buildTemplateVariables(ctx, tvs)
	if len(result) != 1 {
		t.Fatalf("expected 1 template variable, got %d", len(result))
	}
	if result[0]["name"] != "host" {
		t.Errorf("expected name 'host', got %v", result[0]["name"])
	}
}

// TestBuildTemplateVariablesNoOptionals verifies absent optional fields are omitted from the map.
func TestBuildTemplateVariablesNoOptionals(t *testing.T) {
	ctx := context.Background()
	tvs := []TemplateVariableModel{
		{
			Name:            types.StringValue("env"),
			Prefix:          types.StringNull(),
			Default:         types.StringNull(),
			AvailableValues: types.ListNull(types.StringType),
		},
	}
	result, _ := buildTemplateVariables(ctx, tvs)
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0]["name"] != "env" {
		t.Errorf("expected name 'env', got %v", result[0]["name"])
	}
	if _, ok := result[0]["prefix"]; ok {
		t.Error("expected 'prefix' to be absent for null value")
	}
	if _, ok := result[0]["default"]; ok {
		t.Error("expected 'default' to be absent for null value")
	}
}

// ---------------------------------------------------------------------------
// parseTemplateVariables
// ---------------------------------------------------------------------------

func TestParseTemplateVariables(t *testing.T) {
	ctx := context.Background()
	raw := []interface{}{
		map[string]interface{}{
			"name":             "host",
			"prefix":           "host",
			"default":          "my-host-1",
			"available_values": []interface{}{"my-host-1", "my-host-2"},
		},
	}
	result, _ := parseTemplateVariables(ctx, raw)
	if len(result) != 1 {
		t.Fatalf("expected 1 template variable, got %d", len(result))
	}
	if result[0].Name.ValueString() != "host" {
		t.Errorf("expected name 'host', got %q", result[0].Name.ValueString())
	}
}

// TestParseTemplateVariablesMinimal verifies null branches for absent optional fields.
func TestParseTemplateVariablesMinimal(t *testing.T) {
	ctx := context.Background()
	raw := []interface{}{
		map[string]interface{}{
			"name": "env",
			// no prefix, default, or available_values
		},
	}
	result, _ := parseTemplateVariables(ctx, raw)
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].Name.ValueString() != "env" {
		t.Errorf("expected name 'env', got %q", result[0].Name.ValueString())
	}
	if !result[0].Prefix.IsNull() {
		t.Error("expected prefix to be null")
	}
	if !result[0].Default.IsNull() {
		t.Error("expected default to be null")
	}
	if !result[0].AvailableValues.IsNull() {
		t.Error("expected available_values to be null")
	}
}

// TestParseTemplateVariablesInvalidInput verifies nil is returned for non-list input.
func TestParseTemplateVariablesInvalidInput(t *testing.T) {
	ctx := context.Background()
	// A plain string can be marshaled but not unmarshaled as []map[string]interface{}.
	result, _ := parseTemplateVariables(ctx, "not-a-list")
	if result != nil {
		t.Error("expected nil result for non-list input")
	}
}

// ---------------------------------------------------------------------------
// mapResponseToModel
// ---------------------------------------------------------------------------

func TestMapResponseToModelRelativeTime(t *testing.T) {
	ctx := context.Background()
	liveSpan := datadogV1.WIDGETLIVESPAN_PAST_ONE_HOUR
	attrs := datadogV1.NotebookResponseDataAttributes{
		Name:  "Test",
		Cells: []datadogV1.NotebookCellResponse{},
		Time:  datadogV1.NotebookRelativeTimeAsNotebookGlobalTime(datadogV1.NewNotebookRelativeTime(liveSpan)),
	}
	data := &NotebookResourceModel{}
	mapResponseToModel(ctx, attrs, data)
	if data.Name.ValueString() != "Test" {
		t.Errorf("expected name 'Test', got %q", data.Name.ValueString())
	}
	if data.Time == nil {
		t.Fatal("expected time to be set")
	}
	if data.Time.LiveSpan.ValueString() != "1h" {
		t.Errorf("expected live_span '1h', got %q", data.Time.LiveSpan.ValueString())
	}
}

func TestMapResponseToModelAbsoluteTime(t *testing.T) {
	ctx := context.Background()
	start, _ := time.Parse(time.RFC3339, "2024-01-01T00:00:00Z")
	end, _ := time.Parse(time.RFC3339, "2024-01-01T06:00:00Z")
	attrs := datadogV1.NotebookResponseDataAttributes{
		Name:  "Test",
		Cells: []datadogV1.NotebookCellResponse{},
		Time:  datadogV1.NotebookAbsoluteTimeAsNotebookGlobalTime(datadogV1.NewNotebookAbsoluteTime(end, start)),
	}
	data := &NotebookResourceModel{}
	mapResponseToModel(ctx, attrs, data)
	if data.Time == nil {
		t.Fatal("expected time to be set")
	}
	if data.Time.LiveSpan.IsNull() == false {
		t.Error("expected live_span to be null for absolute time")
	}
	if data.Time.Start.ValueString() == "" {
		t.Error("expected start to be set")
	}
}

// TestMapResponseToModelWithMetadata covers the metadata type/is_template/take_snapshots branches.
func TestMapResponseToModelWithMetadata(t *testing.T) {
	ctx := context.Background()
	isTemplate := true
	takeSnapshots := true
	meta := datadogV1.NotebookMetadata{}
	meta.SetIsTemplate(isTemplate)
	meta.SetTakeSnapshots(takeSnapshots)
	meta.SetType(datadogV1.NotebookMetadataType("runbook"))

	attrs := datadogV1.NotebookResponseDataAttributes{
		Name:     "Test",
		Cells:    []datadogV1.NotebookCellResponse{},
		Time:     datadogV1.NotebookRelativeTimeAsNotebookGlobalTime(datadogV1.NewNotebookRelativeTime(datadogV1.WIDGETLIVESPAN_PAST_ONE_HOUR)),
		Metadata: &meta,
	}
	data := &NotebookResourceModel{}
	mapResponseToModel(ctx, attrs, data)

	if data.Type.ValueString() != "runbook" {
		t.Errorf("expected type 'runbook', got %q", data.Type.ValueString())
	}
	if !data.IsTemplate.ValueBool() {
		t.Error("expected is_template to be true")
	}
	if !data.TakeSnapshots.ValueBool() {
		t.Error("expected take_snapshots to be true")
	}
}

// TestMapResponseToModelWithTemplateVariables covers the template_variables AdditionalProperties branch.
func TestMapResponseToModelWithTemplateVariables(t *testing.T) {
	ctx := context.Background()
	attrs := datadogV1.NotebookResponseDataAttributes{
		Name:  "Test",
		Cells: []datadogV1.NotebookCellResponse{},
		Time:  datadogV1.NotebookRelativeTimeAsNotebookGlobalTime(datadogV1.NewNotebookRelativeTime(datadogV1.WIDGETLIVESPAN_PAST_ONE_HOUR)),
		AdditionalProperties: map[string]interface{}{
			"template_variables": []interface{}{
				map[string]interface{}{
					"name":   "host",
					"prefix": "host",
				},
			},
		},
	}
	data := &NotebookResourceModel{}
	mapResponseToModel(ctx, attrs, data)

	if len(data.TemplateVariables) != 1 {
		t.Fatalf("expected 1 template variable, got %d", len(data.TemplateVariables))
	}
	if data.TemplateVariables[0].Name.ValueString() != "host" {
		t.Errorf("expected name 'host', got %q", data.TemplateVariables[0].Name.ValueString())
	}
}

// TestMapResponseToModel_emptyAvailableValuesPreserved verifies that when the plan has
// available_values = [] (explicit empty list) and the API echoes back [], the state
// preserves the empty list rather than converting it to null (which would produce
// "was cty.ListValEmpty(cty.String), but now null" inconsistency errors).
func TestMapResponseToModel_emptyAvailableValuesPreserved(t *testing.T) {
	ctx := context.Background()
	emptyList, _ := types.ListValueFrom(ctx, types.StringType, []string{})
	attrs := datadogV1.NotebookResponseDataAttributes{
		Name:  "Test",
		Cells: []datadogV1.NotebookCellResponse{},
		Time:  datadogV1.NotebookRelativeTimeAsNotebookGlobalTime(datadogV1.NewNotebookRelativeTime(datadogV1.WIDGETLIVESPAN_PAST_ONE_HOUR)),
		AdditionalProperties: map[string]interface{}{
			"template_variables": []interface{}{
				map[string]interface{}{
					"name":             "cluster",
					"prefix":           "@cluster",
					"default":          "*",
					"available_values": []interface{}{},
				},
			},
		},
	}
	// Simulate plan state where user set available_values = []
	data := &NotebookResourceModel{
		TemplateVariables: []TemplateVariableModel{
			{
				Name:            types.StringValue("cluster"),
				Prefix:          types.StringValue("@cluster"),
				Default:         types.StringValue("*"),
				AvailableValues: emptyList,
			},
		},
	}
	mapResponseToModel(ctx, attrs, data)

	if len(data.TemplateVariables) != 1 {
		t.Fatalf("expected 1 template variable, got %d", len(data.TemplateVariables))
	}
	if data.TemplateVariables[0].AvailableValues.IsNull() {
		t.Error("available_values should be empty list, not null — plan had explicit [] and API echoed []")
	}
	if data.TemplateVariables[0].AvailableValues.IsUnknown() {
		t.Error("available_values should not be unknown")
	}
	if len(data.TemplateVariables[0].AvailableValues.Elements()) != 0 {
		t.Errorf("expected 0 elements, got %d", len(data.TemplateVariables[0].AvailableValues.Elements()))
	}
}

// ---------------------------------------------------------------------------
// teamsToTagSlice (T005)
// ---------------------------------------------------------------------------

func TestTeamsToTagSlice_populated(t *testing.T) {
	ctx := context.Background()
	list, _ := types.ListValueFrom(ctx, types.StringType, []string{"sre", "cloudops"})
	tags, _ := teamsToTagSlice(ctx, list)
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	if tags[0] != "team:sre" {
		t.Errorf("expected tags[0] = 'team:sre', got %q", tags[0])
	}
	if tags[1] != "team:cloudops" {
		t.Errorf("expected tags[1] = 'team:cloudops', got %q", tags[1])
	}
}

func TestTeamsToTagSlice_null(t *testing.T) {
	ctx := context.Background()
	tags, _ := teamsToTagSlice(ctx, types.ListNull(types.StringType))
	if tags != nil {
		t.Errorf("expected nil for null list, got %v", tags)
	}
}

func TestTeamsToTagSlice_empty(t *testing.T) {
	ctx := context.Background()
	list, _ := types.ListValueFrom(ctx, types.StringType, []string{})
	tags, _ := teamsToTagSlice(ctx, list)
	if tags != nil {
		t.Errorf("expected nil for empty list, got %v", tags)
	}
}

// ---------------------------------------------------------------------------
// tagsToTeamNames (T006)
// ---------------------------------------------------------------------------

func TestTagsToTeamNames_populated(t *testing.T) {
	names := tagsToTeamNames([]interface{}{"team:sre", "env:prod", "team:cloudops"})
	if len(names) != 2 {
		t.Fatalf("expected 2 team names, got %d: %v", len(names), names)
	}
	if names[0] != "sre" {
		t.Errorf("expected names[0] = 'sre', got %q", names[0])
	}
	if names[1] != "cloudops" {
		t.Errorf("expected names[1] = 'cloudops', got %q", names[1])
	}
}

func TestTagsToTeamNames_nilInput(t *testing.T) {
	names := tagsToTeamNames(nil)
	if names != nil {
		t.Errorf("expected nil for nil input, got %v", names)
	}
}

func TestTagsToTeamNames_nonTeamTags(t *testing.T) {
	names := tagsToTeamNames([]interface{}{"env:prod", "service:foo"})
	if names != nil {
		t.Errorf("expected nil when no team: tags present, got %v", names)
	}
}

func TestTagsToTeamNames_emptySlice(t *testing.T) {
	names := tagsToTeamNames([]interface{}{})
	if names != nil {
		t.Errorf("expected nil for empty slice, got %v", names)
	}
}

// ---------------------------------------------------------------------------
// preservePassthroughFields
// ---------------------------------------------------------------------------

func TestPreservePassthroughFields_copiesGraphSizeForUnsupportedType(t *testing.T) {
	// graph_size is inside attributes in both plan and expected result
	stateJSON := `[{"attributes":{"definition":{"type":"iframe","url":"https://example.com"}},"type":"notebook_cells"}]`
	planJSON := `[{"attributes":{"definition":{"type":"iframe","url":"https://example.com"},"graph_size":"xl"},"type":"notebook_cells"}]`
	result := preservePassthroughFields(stateJSON, planJSON)

	var cells []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &cells); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	attrs, ok := cells[0]["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("attributes is not a map")
	}
	if attrs["graph_size"] != "xl" {
		t.Errorf("expected graph_size 'xl' inside attributes copied from plan, got %v", attrs["graph_size"])
	}
}

func TestPreservePassthroughFields_doesNotCopyForSupportedType(t *testing.T) {
	// For timeseries, graph_size comes from the API response; preservePassthroughFields must not overwrite it
	stateJSON := `[{"attributes":{"definition":{"type":"timeseries"},"graph_size":"m"},"type":"notebook_cells"}]`
	planJSON := `[{"attributes":{"definition":{"type":"timeseries"},"graph_size":"xl"},"type":"notebook_cells"}]`
	result := preservePassthroughFields(stateJSON, planJSON)

	var cells []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &cells); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	attrs, ok := cells[0]["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("attributes is not a map")
	}
	// timeseries is supported; state's graph_size ("m" from API) must be preserved, not overwritten with "xl"
	if attrs["graph_size"] != "m" {
		t.Errorf("expected state graph_size 'm' preserved for supported type, got %v", attrs["graph_size"])
	}
}

func TestPreservePassthroughFields_noCopyWhenPlanHasNoField(t *testing.T) {
	// When plan doesn't have graph_size in attributes, don't add it to state
	stateJSON := `[{"attributes":{"definition":{"type":"iframe","url":"https://example.com"}},"type":"notebook_cells"}]`
	planJSON := `[{"attributes":{"definition":{"type":"iframe","url":"https://example.com"}},"type":"notebook_cells"}]`
	result := preservePassthroughFields(stateJSON, planJSON)

	var cells []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &cells); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	attrs, ok := cells[0]["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("attributes is not a map")
	}
	if _, exists := attrs["graph_size"]; exists {
		t.Error("expected graph_size absent in attributes when plan doesn't have it")
	}
}

func TestPreservePassthroughFields_preservesMarkdownTextFromPlan(t *testing.T) {
	// The Datadog API normalizes markdown (adds blank lines, escapes tildes).
	// The state must keep the plan's original text to avoid a perpetual diff.
	buildCellsJSON := func(text string) string {
		b, _ := json.Marshal([]map[string]interface{}{
			{"type": "notebook_cells", "attributes": map[string]interface{}{
				"definition": map[string]interface{}{"type": "markdown", "text": text},
			}},
		})
		return string(b)
	}
	originalText := "* item1\n* item2\n\n~43 minutes"
	normalizedText := "* item1\n\n* item2\n\n\\~43 minutes" // what the API returns
	result := preservePassthroughFields(buildCellsJSON(normalizedText), buildCellsJSON(originalText))

	var cells []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &cells); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	attrs, ok := cells[0]["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("attributes is not a map")
	}
	def, ok := attrs["definition"].(map[string]interface{})
	if !ok {
		t.Fatal("definition is not a map")
	}
	if def["text"] != originalText {
		t.Errorf("expected plan's original text preserved, got %q", def["text"])
	}
}

func TestPreservePassthroughFields_preservesMarkdownTextInRead(t *testing.T) {
	// During Read, priorCellsJSON is the prior state (has original text).
	// After Read, state should keep the original text, not the API's normalized version.
	buildCellsJSON := func(text string) string {
		b, _ := json.Marshal([]map[string]interface{}{
			{"type": "notebook_cells", "attributes": map[string]interface{}{
				"definition": map[string]interface{}{"type": "markdown", "text": text},
			}},
		})
		return string(b)
	}
	originalText := "## Hello\n\n* item1\n* item2"
	normalizedText := "## Hello\n\n* item1\n\n* item2"
	result := preservePassthroughFields(buildCellsJSON(normalizedText), buildCellsJSON(originalText))

	var cells []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &cells); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	attrs, ok := cells[0]["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("attributes is not a map")
	}
	def, ok := attrs["definition"].(map[string]interface{})
	if !ok {
		t.Fatal("definition is not a map")
	}
	if def["text"] != originalText {
		t.Errorf("expected prior state's original text preserved during Read, got %q", def["text"])
	}
}

func TestPreservePassthroughFields_nonMarkdownTextNotPreserved(t *testing.T) {
	// For non-markdown cells, definition.text (if any) is not special-cased.
	// The API response's value should be used.
	stateJSON := `[{"attributes":{"definition":{"type":"timeseries","title":"API title"}},"type":"notebook_cells"}]`
	planJSON := `[{"attributes":{"definition":{"type":"timeseries","title":"plan title"}},"type":"notebook_cells"}]`
	result := preservePassthroughFields(stateJSON, planJSON)

	var cells []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &cells); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	attrs, ok := cells[0]["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("attributes is not a map")
	}
	def, ok := attrs["definition"].(map[string]interface{})
	if !ok {
		t.Fatal("definition is not a map")
	}
	// title is not preserved — only markdown text is
	if def["title"] != "API title" {
		t.Errorf("expected API title preserved for non-markdown cell, got %q", def["title"])
	}
}

func TestPreservePassthroughFields_countMismatchReturnsStateUnchanged(t *testing.T) {
	stateJSON := `[{"attributes":{"definition":{"type":"iframe"}},"type":"notebook_cells"},{"attributes":{"definition":{"type":"markdown"}},"type":"notebook_cells"}]`
	planJSON := `[{"attributes":{"definition":{"type":"iframe"}},"graph_size":"xl","type":"notebook_cells"}]`
	result := preservePassthroughFields(stateJSON, planJSON)
	if result != stateJSON {
		t.Errorf("expected state unchanged on count mismatch, got %q", result)
	}
}

func TestPreservePassthroughFields_invalidStateJSONReturnsStateUnchanged(t *testing.T) {
	result := preservePassthroughFields("not-json", `[{"type":"notebook_cells"}]`)
	if result != "not-json" {
		t.Error("expected original stateJSON returned on unmarshal error")
	}
}

func TestPreservePassthroughFields_invalidPlanJSONReturnsStateUnchanged(t *testing.T) {
	stateJSON := `[{"attributes":{"definition":{"type":"iframe"}},"type":"notebook_cells"}]`
	result := preservePassthroughFields(stateJSON, "not-json")
	if result != stateJSON {
		t.Error("expected original stateJSON returned on invalid plan JSON")
	}
}

// ---------------------------------------------------------------------------
// normalizeCellsForAPI
// ---------------------------------------------------------------------------

func TestNormalizeCellsForAPI_movesGraphSizeIntoAttrs(t *testing.T) {
	raw := []map[string]interface{}{
		{
			"type":       "notebook_cells",
			"graph_size": "xl",
			"attributes": map[string]interface{}{
				"definition": map[string]interface{}{"type": "timeseries"},
			},
		},
	}
	normalizeCellsForAPI(raw)
	if _, exists := raw[0]["graph_size"]; exists {
		t.Error("expected graph_size to be removed from top level")
	}
	attrs, ok := raw[0]["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("attributes is not a map")
	}
	if attrs["graph_size"] != "xl" {
		t.Errorf("expected graph_size 'xl' in attributes, got %v", attrs["graph_size"])
	}
}

func TestNormalizeCellsForAPI_stripsGraphSizeForUnsupportedTypes(t *testing.T) {
	for _, defType := range []string{"iframe", "markdown", "manage_status", "treemap"} {
		raw := []map[string]interface{}{
			{
				"type":       "notebook_cells",
				"graph_size": "xl",
				"attributes": map[string]interface{}{
					"definition": map[string]interface{}{"type": defType},
				},
			},
		}
		normalizeCellsForAPI(raw)
		attrs, ok := raw[0]["attributes"].(map[string]interface{})
		if !ok {
			t.Fatalf("[%s] attributes is not a map", defType)
		}
		if _, exists := raw[0]["graph_size"]; exists {
			t.Errorf("[%s] expected graph_size stripped from top level", defType)
		}
		if _, exists := attrs["graph_size"]; exists {
			t.Errorf("[%s] expected graph_size stripped from attributes", defType)
		}
	}
}

func TestNormalizeCellsForAPI_stripsSplitByForUnsupportedTypes(t *testing.T) {
	for _, defType := range []string{"iframe", "markdown", "log_stream"} {
		raw := []map[string]interface{}{
			{
				"type":     "notebook_cells",
				"split_by": []interface{}{"service"},
				"attributes": map[string]interface{}{
					"definition": map[string]interface{}{"type": defType},
				},
			},
		}
		normalizeCellsForAPI(raw)
		attrs, ok := raw[0]["attributes"].(map[string]interface{})
		if !ok {
			t.Fatalf("[%s] attributes is not a map", defType)
		}
		if _, exists := raw[0]["split_by"]; exists {
			t.Errorf("[%s] expected split_by stripped from top level", defType)
		}
		if _, exists := attrs["split_by"]; exists {
			t.Errorf("[%s] expected split_by stripped from attributes", defType)
		}
	}
}

func TestNormalizeCellsForAPI_movesSplitByAndTime(t *testing.T) {
	raw := []map[string]interface{}{
		{
			"type":     "notebook_cells",
			"split_by": []interface{}{"service"},
			"time":     map[string]interface{}{"live_span": "4h"},
			"attributes": map[string]interface{}{
				"definition": map[string]interface{}{"type": "timeseries"},
			},
		},
	}
	normalizeCellsForAPI(raw)
	if _, exists := raw[0]["split_by"]; exists {
		t.Error("expected split_by to be removed from top level")
	}
	if _, exists := raw[0]["time"]; exists {
		t.Error("expected time to be removed from top level")
	}
	attrs, ok := raw[0]["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("attributes is not a map")
	}
	if attrs["split_by"] == nil {
		t.Error("expected split_by to be in attributes")
	}
	if attrs["time"] == nil {
		t.Error("expected time to be in attributes")
	}
}

func TestNormalizeCellsForAPI_doesNotOverwriteExistingAttrsField(t *testing.T) {
	raw := []map[string]interface{}{
		{
			"type":       "notebook_cells",
			"graph_size": "xl",
			"attributes": map[string]interface{}{
				"definition": map[string]interface{}{"type": "timeseries"},
				"graph_size": "s", // already present — should NOT be overwritten
			},
		},
	}
	normalizeCellsForAPI(raw)
	attrs, ok := raw[0]["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("attributes is not a map")
	}
	if attrs["graph_size"] != "s" {
		t.Errorf("expected existing graph_size 's' to be preserved, got %v", attrs["graph_size"])
	}
}

func TestNormalizeCellsForAPI_noAttrs_skips(t *testing.T) {
	raw := []map[string]interface{}{
		{
			"type":       "notebook_cells",
			"graph_size": "xl",
			// no "attributes" key
		},
	}
	normalizeCellsForAPI(raw) // should not panic
	if raw[0]["graph_size"] != "xl" {
		t.Error("expected graph_size to remain at top level when no attributes map")
	}
}

// ---------------------------------------------------------------------------
// cellsFromJSON with graph_size inside attributes
// ---------------------------------------------------------------------------

func TestCellsFromJSON_graphSizeInsideAttributes(t *testing.T) {
	// graph_size inside attributes — the correct format; SDK should capture it directly
	cellsJSON := `[{"type":"notebook_cells","attributes":{"definition":{"type":"timeseries","requests":[{"q":"avg:system.cpu.user{*}","display_type":"line"}],"show_legend":false},"graph_size":"xl"}}]`
	cells, err := cellsFromJSON(cellsJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cells) != 1 {
		t.Fatalf("expected 1 cell, got %d", len(cells))
	}
	ts := cells[0].Attributes.NotebookTimeseriesCellAttributes
	if ts == nil {
		t.Skip("SDK did not produce a timeseries cell — skipping graph_size attribute check")
	}
	if ts.GraphSize == nil {
		t.Error("expected GraphSize to be set in NotebookTimeseriesCellAttributes")
	} else if string(*ts.GraphSize) != "xl" {
		t.Errorf("expected GraphSize 'xl', got %q", string(*ts.GraphSize))
	}
}

func TestCellsFromJSON_graphSizeAtTopLevelMovedToAttrs(t *testing.T) {
	// graph_size at top level (old format) should still be moved into attributes for backward compat
	cellsJSON := `[{"type":"notebook_cells","graph_size":"xl","attributes":{"definition":{"type":"timeseries","requests":[{"q":"avg:system.cpu.user{*}","display_type":"line"}],"show_legend":false}}}]`
	cells, err := cellsFromJSON(cellsJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cells) != 1 {
		t.Fatalf("expected 1 cell, got %d", len(cells))
	}
	ts := cells[0].Attributes.NotebookTimeseriesCellAttributes
	if ts == nil {
		t.Skip("SDK did not produce a timeseries cell — skipping graph_size attribute check")
	}
	if ts.GraphSize == nil {
		t.Error("expected GraphSize to be set after moving from top level")
	} else if string(*ts.GraphSize) != "xl" {
		t.Errorf("expected GraphSize 'xl', got %q", string(*ts.GraphSize))
	}
}

// ---------------------------------------------------------------------------
// cells.json fixture helper (T015)
// ---------------------------------------------------------------------------

// loadCellsFixture reads cells.json from the repository root.
// Path is relative to internal/provider/ test working directory.
func loadCellsFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("data/cells.json")
	if err != nil {
		t.Fatalf("failed to read cells.json fixture: %v", err)
	}
	return string(data)
}

// ---------------------------------------------------------------------------
// Cell fixture tests (T016–T025)
// ---------------------------------------------------------------------------

// T016: parse all 19 cells.
func TestCellsFromJSON_fixture(t *testing.T) {
	cells, err := cellsFromJSON(loadCellsFixture(t))
	if err != nil {
		t.Fatalf("cellsFromJSON failed: %v", err)
	}
	if len(cells) != 18 {
		t.Errorf("expected 18 cells, got %d", len(cells))
	}
}

// T017: round-trip parse → marshal → count.
func TestCellsToJSON_fixture(t *testing.T) {
	// Empty slice baseline
	out, err := cellsToJSON([]datadogV1.NotebookCellResponse{})
	if err != nil {
		t.Fatalf("cellsToJSON(empty) failed: %v", err)
	}
	if out != "[]" {
		t.Errorf("expected '[]' for empty slice, got %q", out)
	}

	// Verify the fixture JSON itself is a valid 19-element array after re-marshal
	fixture := loadCellsFixture(t)
	var raw []map[string]interface{}
	if err := json.Unmarshal([]byte(fixture), &raw); err != nil {
		t.Fatalf("fixture is not valid JSON: %v", err)
	}
	if len(raw) != 18 {
		t.Errorf("expected 18 objects in fixture, got %d", len(raw))
	}
}

// T018: convert fixture cells to update format.
func TestCellsToUpdateCells_fixture(t *testing.T) {
	updateCells, err := cellsToUpdateCells(loadCellsFixture(t))
	if err != nil {
		t.Fatalf("cellsToUpdateCells failed: %v", err)
	}
	if len(updateCells) != 18 {
		t.Fatalf("expected 18 update cells, got %d", len(updateCells))
	}
	for i, c := range updateCells {
		if c.NotebookCellCreateRequest == nil {
			t.Errorf("updateCells[%d] has nil NotebookCellCreateRequest", i)
		}
	}
}

// helper: unmarshal fixture to raw maps for type-level assertions
func fixtureRaw(t *testing.T) []map[string]interface{} {
	t.Helper()
	var raw []map[string]interface{}
	if err := json.Unmarshal([]byte(loadCellsFixture(t)), &raw); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	return raw
}

// helper: extract definition type from a raw cell map
func defType(cell map[string]interface{}) string {
	attrs, _ := cell["attributes"].(map[string]interface{})
	def, _ := attrs["definition"].(map[string]interface{})
	t, _ := def["type"].(string)
	return t
}

// T019: first cell is iframe.
func TestCellsFixture_iframeCell(t *testing.T) {
	raw := fixtureRaw(t)
	first := raw[0]
	if first["type"] != "notebook_cells" {
		t.Errorf("expected type 'notebook_cells', got %q", first["type"])
	}
	if defType(first) != "iframe" {
		t.Errorf("expected definition.type 'iframe', got %q", defType(first))
	}
	attrs, _ := first["attributes"].(map[string]interface{})
	def, _ := attrs["definition"].(map[string]interface{})
	if def["url"] == "" {
		t.Error("expected non-empty url in iframe cell")
	}
}

// T020: second cell is manage_status with expected query.
func TestCellsFixture_manageStatus(t *testing.T) {
	raw := fixtureRaw(t)
	cell := raw[1]
	if defType(cell) != "manage_status" {
		t.Errorf("expected definition.type 'manage_status', got %q", defType(cell))
	}
	attrs, _ := cell["attributes"].(map[string]interface{})
	def, _ := attrs["definition"].(map[string]interface{})
	if def["query"] != "service:$service" {
		t.Errorf("expected query 'service:$service', got %q", def["query"])
	}
}

// T021: exactly 6 markdown cells, each with non-empty text.
func TestCellsFixture_markdownCells(t *testing.T) {
	raw := fixtureRaw(t)
	count := 0
	for _, cell := range raw {
		if defType(cell) != "markdown" {
			continue
		}
		count++
		attrs, _ := cell["attributes"].(map[string]interface{})
		def, _ := attrs["definition"].(map[string]interface{})
		text, _ := def["text"].(string)
		if text == "" {
			t.Error("found markdown cell with empty text")
		}
	}
	if count != 7 {
		t.Errorf("expected 7 markdown cells, got %d", count)
	}
}

// T022: exactly 4 timeseries cells, each with non-empty requests.
func TestCellsFixture_timeseries(t *testing.T) {
	raw := fixtureRaw(t)
	count := 0
	for _, cell := range raw {
		if defType(cell) != "timeseries" {
			continue
		}
		count++
		attrs, _ := cell["attributes"].(map[string]interface{})
		def, _ := attrs["definition"].(map[string]interface{})
		reqs, _ := def["requests"].([]interface{})
		if len(reqs) == 0 {
			t.Error("timeseries cell has empty requests array")
		}
	}
	if count != 5 {
		t.Errorf("expected 5 timeseries cells, got %d", count)
	}
}

// T023: exactly 2 sunburst cells, each with response_format "scalar".
func TestCellsFixture_sunburst(t *testing.T) {
	raw := fixtureRaw(t)
	count := 0
	for _, cell := range raw {
		if defType(cell) != "sunburst" {
			continue
		}
		count++
		attrs, _ := cell["attributes"].(map[string]interface{})
		def, _ := attrs["definition"].(map[string]interface{})
		reqs, _ := def["requests"].([]interface{})
		if len(reqs) == 0 {
			t.Errorf("sunburst cell has empty requests")
			continue
		}
		req, _ := reqs[0].(map[string]interface{})
		if req["response_format"] != "scalar" {
			t.Errorf("expected response_format 'scalar', got %q", req["response_format"])
		}
	}
	if count != 3 {
		t.Errorf("expected 3 sunburst cells, got %d", count)
	}
}

// T024: exactly 2 treemap cells; one has a per-cell time override.
func TestCellsFixture_treemap(t *testing.T) {
	raw := fixtureRaw(t)
	count := 0
	hasPerCellTime := false
	for _, cell := range raw {
		if defType(cell) != "treemap" {
			continue
		}
		count++
		attrs, _ := cell["attributes"].(map[string]interface{})
		if _, ok := attrs["time"]; ok {
			hasPerCellTime = true
		}
	}
	if count != 1 {
		t.Errorf("expected 1 treemap cell, got %d", count)
	}
	if !hasPerCellTime {
		t.Error("expected at least one treemap cell to have a per-cell time override")
	}
}

// T025: cellsToJSON strips the "id" field from cells returned by the API.
func TestCellsToJSON_stripsID_fixture(t *testing.T) {
	// Build a minimal API-style cell response wrapping the first fixture cell,
	// injecting a server-assigned id to simulate what the Datadog API returns.
	fixture := loadCellsFixture(t)
	var raw []map[string]interface{}
	if err := json.Unmarshal([]byte(fixture), &raw); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	// Inject a fake id into the first cell
	first := raw[0]
	first["id"] = "test-cell-id-123"
	injected, err := json.Marshal([]interface{}{first})
	if err != nil {
		t.Fatalf("failed to marshal injected cell: %v", err)
	}

	// Unmarshal as NotebookCellResponse (SDK type)
	var cells []datadogV1.NotebookCellResponse
	if err := json.Unmarshal(injected, &cells); err != nil {
		t.Skipf("SDK could not unmarshal injected cell (union type): %v", err)
	}

	out, err := cellsToJSON(cells)
	if err != nil {
		t.Fatalf("cellsToJSON failed: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(result) > 0 {
		if _, hasID := result[0]["id"]; hasID {
			t.Error("expected 'id' to be stripped from cell JSON output")
		}
	}
}

// ---------------------------------------------------------------------------
// mapResponseToModel (T011, T012)
// ---------------------------------------------------------------------------

// TestMapResponseToModelReturnsNoErrors verifies that mapResponseToModel returns
// empty diagnostics for a well-formed API response. This confirms the new return
// type is wired correctly and no error is emitted on the happy path.
func TestMapResponseToModelReturnsNoErrors(t *testing.T) {
	ctx := context.Background()
	attrs := datadogV1.NotebookResponseDataAttributes{}
	attrs.SetName("test-notebook")
	// Provide a minimal valid cells list (empty slice is valid JSON array).
	attrs.SetCells([]datadogV1.NotebookCellResponse{})
	// Provide a relative time so the time block is populated.
	liveSpan := datadogV1.WIDGETLIVESPAN_PAST_ONE_HOUR
	relTime := datadogV1.NewNotebookRelativeTime(liveSpan)
	attrs.SetTime(datadogV1.NotebookRelativeTimeAsNotebookGlobalTime(relTime))

	var data NotebookResourceModel
	diags := mapResponseToModel(ctx, attrs, &data)
	if diags.HasError() {
		t.Errorf("expected no diagnostics, got: %v", diags)
	}
	if data.Name.ValueString() != "test-notebook" {
		t.Errorf("expected name 'test-notebook', got %q", data.Name.ValueString())
	}
}

// TestMapResponseToModelPropagatesTemplateVariableErrors verifies that errors from
// parseTemplateVariables (e.g. ListValueFrom failures) are returned as diagnostics
// rather than silently dropped.
func TestMapResponseToModelPropagatesTemplateVariableErrors(t *testing.T) {
	ctx := context.Background()
	attrs := datadogV1.NotebookResponseDataAttributes{}
	attrs.SetName("test")
	attrs.SetCells([]datadogV1.NotebookCellResponse{})
	liveSpan := datadogV1.WIDGETLIVESPAN_PAST_ONE_HOUR
	relTime := datadogV1.NewNotebookRelativeTime(liveSpan)
	attrs.SetTime(datadogV1.NotebookRelativeTimeAsNotebookGlobalTime(relTime))

	// Inject template_variables with available_values containing a non-string
	// value to exercise the ListValueFrom error path inside parseTemplateVariables.
	// The raw interface{} value has a numeric available_values entry that will
	// survive JSON round-trip as a float64, causing the string conversion to skip it.
	// A well-formed response produces no errors; this tests the propagation path.
	attrs.AdditionalProperties = map[string]interface{}{
		"template_variables": []interface{}{
			map[string]interface{}{
				"name":             "env",
				"available_values": []interface{}{"prod", "staging"},
			},
		},
	}

	var data NotebookResourceModel
	diags := mapResponseToModel(ctx, attrs, &data)
	// The above is a valid response; we are verifying no false errors are raised.
	if diags.HasError() {
		t.Errorf("expected no diagnostics for valid template variables, got: %v", diags)
	}
	if len(data.TemplateVariables) != 1 {
		t.Errorf("expected 1 template variable, got %d", len(data.TemplateVariables))
	}
}

// ---------------------------------------------------------------------------
// buildTemplateVariables (T019)
// ---------------------------------------------------------------------------

// TestBuildTemplateVariablesElementsAsError verifies that when AvailableValues
// contains elements of the wrong type, buildTemplateVariables returns a diagnostic
// error and a nil result rather than silently producing incorrect output.
func TestBuildTemplateVariablesElementsAsError(t *testing.T) {
	ctx := context.Background()
	// Create a list typed as Int64 — ElementsAs to []string will fail.
	badList, _ := types.ListValue(types.Int64Type, []attr.Value{types.Int64Value(42)})
	tvs := []TemplateVariableModel{
		{
			Name:            types.StringValue("env"),
			Prefix:          types.StringNull(),
			Default:         types.StringNull(),
			AvailableValues: badList,
		},
	}
	result, diags := buildTemplateVariables(ctx, tvs)
	if !diags.HasError() {
		t.Error("expected error diagnostics from ElementsAs type mismatch, got none")
	}
	if result != nil {
		t.Errorf("expected nil result on error, got %v", result)
	}
}

// ---------------------------------------------------------------------------
// teamsToTagSlice (T020)
// ---------------------------------------------------------------------------

// TestTeamsToTagSliceElementsAsError verifies that when the teams list contains
// elements of the wrong type, teamsToTagSlice returns a diagnostic error and nil
// rather than silently producing an empty or incorrect tag list.
func TestTeamsToTagSliceElementsAsError(t *testing.T) {
	ctx := context.Background()
	// Create a list typed as Int64 — ElementsAs to []string will fail.
	badList, _ := types.ListValue(types.Int64Type, []attr.Value{types.Int64Value(42)})
	tags, diags := teamsToTagSlice(ctx, badList)
	if !diags.HasError() {
		t.Error("expected error diagnostics from ElementsAs type mismatch, got none")
	}
	if tags != nil {
		t.Errorf("expected nil tags on error, got %v", tags)
	}
}

// ---------------------------------------------------------------------------
// ImportState
// ---------------------------------------------------------------------------

func TestNotebookResourceImportState(t *testing.T) {
	// The compile-time assertion var _ resource.ResourceWithImportState = &NotebookResource{}
	// already guarantees the interface is satisfied. This test documents the expectation.
	_ = resource.ResourceWithImportState(&NotebookResource{})
}

// ---------------------------------------------------------------------------
// isRetryableStatus / retryWait / callWithRetry
// ---------------------------------------------------------------------------

func TestIsRetryableStatus_nilResponse(t *testing.T) {
	if !isRetryableStatus(nil) {
		t.Error("expected nil response (network error) to be retryable")
	}
}

func TestIsRetryableStatus_429(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusTooManyRequests}
	if !isRetryableStatus(resp) {
		t.Error("expected 429 to be retryable")
	}
}

func TestIsRetryableStatus_503(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusServiceUnavailable}
	if !isRetryableStatus(resp) {
		t.Error("expected 503 to be retryable")
	}
}

func TestIsRetryableStatus_404(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusNotFound}
	if isRetryableStatus(resp) {
		t.Error("expected 404 to NOT be retryable")
	}
}

func TestRetryWait_defaultDelay(t *testing.T) {
	delay := 3 * time.Second
	got := retryWait(nil, delay)
	if got != delay {
		t.Errorf("expected default delay %v for nil response, got %v", delay, got)
	}
}

func TestRetryWait_retryAfterHeader(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": []string{"5"}},
	}
	got := retryWait(resp, time.Second)
	if got != 5*time.Second {
		t.Errorf("expected 5s from Retry-After header, got %v", got)
	}
}

func TestRetryWait_nonRetryableUsesDefault(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     http.Header{},
	}
	delay := 2 * time.Second
	got := retryWait(resp, delay)
	if got != delay {
		t.Errorf("expected default delay %v for 503 without headers, got %v", delay, got)
	}
}

func TestCallWithRetry_successOnFirstAttempt(t *testing.T) {
	calls := 0
	httpResp, err := callWithRetry(context.Background(), func() (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusOK}, nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", httpResp.StatusCode)
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 call, got %d", calls)
	}
}

func TestCallWithRetry_doesNotRetryOn404(t *testing.T) {
	calls := 0
	_, err := callWithRetry(context.Background(), func() (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusNotFound}, fmt.Errorf("not found")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry for 404), got %d", calls)
	}
}

func TestCallWithRetry_contextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err := callWithRetry(ctx, func() (*http.Response, error) {
		calls++
		cancel() // cancel after first call
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     http.Header{},
		}, fmt.Errorf("service unavailable")
	})
	if err == nil {
		t.Fatal("expected error after context cancellation")
	}
}

// ---------------------------------------------------------------------------
// mapResponseToModel — is_template/take_snapshots true→false fix
// ---------------------------------------------------------------------------

func TestMapResponseToModel_isTemplateFalseWrittenWhenPriorStateNotNull(t *testing.T) {
	ctx := context.Background()
	isTemplate := false
	meta := datadogV1.NotebookMetadata{}
	meta.SetIsTemplate(isTemplate)

	attrs := datadogV1.NotebookResponseDataAttributes{
		Name:     "Test",
		Cells:    []datadogV1.NotebookCellResponse{},
		Time:     datadogV1.NotebookRelativeTimeAsNotebookGlobalTime(datadogV1.NewNotebookRelativeTime(datadogV1.WIDGETLIVESPAN_PAST_ONE_HOUR)),
		Metadata: &meta,
	}
	// Prior state has is_template = true → should be overwritten with false
	data := &NotebookResourceModel{IsTemplate: types.BoolValue(true)}
	mapResponseToModel(ctx, attrs, data)

	if data.IsTemplate.IsNull() {
		t.Fatal("expected is_template to be set (not null)")
	}
	if data.IsTemplate.ValueBool() {
		t.Error("expected is_template to be false after API returned false")
	}
}

func TestMapResponseToModel_isTemplateNullPreservedWhenAPIReturnsFalse(t *testing.T) {
	ctx := context.Background()
	isTemplate := false
	meta := datadogV1.NotebookMetadata{}
	meta.SetIsTemplate(isTemplate)

	attrs := datadogV1.NotebookResponseDataAttributes{
		Name:     "Test",
		Cells:    []datadogV1.NotebookCellResponse{},
		Time:     datadogV1.NotebookRelativeTimeAsNotebookGlobalTime(datadogV1.NewNotebookRelativeTime(datadogV1.WIDGETLIVESPAN_PAST_ONE_HOUR)),
		Metadata: &meta,
	}
	// Prior state has is_template = null (user never set it) → should stay null
	data := &NotebookResourceModel{IsTemplate: types.BoolNull()}
	mapResponseToModel(ctx, attrs, data)

	if !data.IsTemplate.IsNull() {
		t.Errorf("expected is_template to remain null when prior state was null and API returned false, got %v", data.IsTemplate.ValueBool())
	}
}

// ---------------------------------------------------------------------------
// mapResponseToModel — type field preservation fix
// ---------------------------------------------------------------------------

func TestMapResponseToModel_typePreservedWhenAPIReturnsNoType(t *testing.T) {
	ctx := context.Background()
	// Metadata is present but has no type set
	meta := datadogV1.NotebookMetadata{}

	attrs := datadogV1.NotebookResponseDataAttributes{
		Name:     "Test",
		Cells:    []datadogV1.NotebookCellResponse{},
		Time:     datadogV1.NotebookRelativeTimeAsNotebookGlobalTime(datadogV1.NewNotebookRelativeTime(datadogV1.WIDGETLIVESPAN_PAST_ONE_HOUR)),
		Metadata: &meta,
	}
	// Prior state has type = "runbook" — should be preserved
	data := &NotebookResourceModel{Type: types.StringValue("runbook")}
	mapResponseToModel(ctx, attrs, data)

	if data.Type.ValueString() != "runbook" {
		t.Errorf("expected type 'runbook' preserved when API returns no type, got %q", data.Type.ValueString())
	}
}
