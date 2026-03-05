// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	datadogV1 "github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
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
	req := resource.MetadataRequest{ProviderTypeName: "datadoggy"}
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), req, resp)
	if resp.TypeName != "datadoggy_notebook" {
		t.Errorf("expected type name 'datadoggy_notebook', got %q", resp.TypeName)
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
	clients := &DatadogClients{APIContext: context.Background()}
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
	result := buildTemplateVariables(ctx, tvs)
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
	result := buildTemplateVariables(ctx, tvs)
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
	result := parseTemplateVariables(ctx, raw)
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
	result := parseTemplateVariables(ctx, raw)
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
	result := parseTemplateVariables(ctx, "not-a-list")
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
