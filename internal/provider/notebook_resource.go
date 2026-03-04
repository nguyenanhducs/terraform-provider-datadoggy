// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	datadogV1 "github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Compile-time interface assertions.
var _ resource.Resource = &NotebookResource{}
var _ resource.ResourceWithConfigure = &NotebookResource{}
var _ resource.ResourceWithImportState = &NotebookResource{}
var _ resource.ResourceWithConfigValidators = &NotebookResource{}

// NewNotebookResource returns a new NotebookResource.
func NewNotebookResource() resource.Resource {
	return &NotebookResource{}
}

// NotebookResource manages Datadog Notebook resources.
type NotebookResource struct {
	notebooksAPI *datadogV1.NotebooksApi
	apiCtx       context.Context
}

// NotebookResourceModel is the Terraform state model for a datadog_notebook resource.
type NotebookResourceModel struct {
	ID                types.String            `tfsdk:"id"`
	Name              types.String            `tfsdk:"name"`
	Cells             types.String            `tfsdk:"cells"`
	Type              types.String            `tfsdk:"type"`
	IsTemplate        types.Bool              `tfsdk:"is_template"`
	TakeSnapshots     types.Bool              `tfsdk:"take_snapshots"`
	Teams             types.List              `tfsdk:"teams"`
	TemplateVariables []TemplateVariableModel `tfsdk:"template_variables"`
	Time              *NotebookTimeModel      `tfsdk:"time"`
}

// TemplateVariableModel represents a notebook template variable.
type TemplateVariableModel struct {
	Name            types.String `tfsdk:"name"`
	Prefix          types.String `tfsdk:"prefix"`
	Default         types.String `tfsdk:"default"`
	AvailableValues types.List   `tfsdk:"available_values"`
}

// NotebookTimeModel represents the global time window for a notebook.
type NotebookTimeModel struct {
	LiveSpan types.String `tfsdk:"live_span"`
	Start    types.String `tfsdk:"start"`
	End      types.String `tfsdk:"end"`
}

// jsonValidator validates that a string attribute is valid JSON.
type jsonValidator struct{}

func (v jsonValidator) Description(_ context.Context) string {
	return "value must be a valid JSON string"
}

func (v jsonValidator) MarkdownDescription(_ context.Context) string {
	return "value must be a valid JSON string"
}

func (v jsonValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	var js json.RawMessage
	if err := json.Unmarshal([]byte(req.ConfigValue.ValueString()), &js); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid JSON",
			fmt.Sprintf("cells must be a valid JSON string: %s", err),
		)
	}
}

// Metadata returns the resource type name.
func (r *NotebookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notebook"
}

// Schema returns the resource schema.
func (r *NotebookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Datadog Notebook resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Notebook ID assigned by Datadog (numeric, stored as string).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable notebook name.",
				Required:            true,
			},
			"cells": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded list of cell objects. Use `jsonencode([...])` to construct.",
				Required:            true,
				Validators: []validator.String{
					jsonValidator{},
				},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Notebook classification. Allowed values: `postmortem`, `runbook`, `investigation`, `documentation`, `report`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("postmortem", "runbook", "investigation", "documentation", "report"),
				},
			},
			"is_template": schema.BoolAttribute{
				MarkdownDescription: "Whether this notebook is a template. Defaults to `false`.",
				Optional:            true,
			},
			"take_snapshots": schema.BoolAttribute{
				MarkdownDescription: "Whether to create graph snapshots. Defaults to `false`.",
				Optional:            true,
			},
			"teams": schema.ListAttribute{
				MarkdownDescription: "Team tags associated with the notebook. Maximum 5 entries.",
				Optional:            true,
				ElementType:         types.StringType,
				Validators: []validator.List{
					listvalidator.SizeAtMost(5),
				},
			},
			"template_variables": schema.ListNestedAttribute{
				MarkdownDescription: "Template variables scoped to this notebook.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Variable name, referenced as `$name` in queries.",
							Required:            true,
						},
						"prefix": schema.StringAttribute{
							MarkdownDescription: "Tag key to filter on (e.g., `host`).",
							Optional:            true,
						},
						"default": schema.StringAttribute{
							MarkdownDescription: "Default filter value.",
							Optional:            true,
						},
						"available_values": schema.ListAttribute{
							MarkdownDescription: "Constrained set of allowed values.",
							Optional:            true,
							ElementType:         types.StringType,
						},
					},
				},
			},
			"time": schema.SingleNestedAttribute{
				MarkdownDescription: "Global time window applied to all cells. Use `live_span` for relative time or `start`+`end` for absolute.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"live_span": schema.StringAttribute{
						MarkdownDescription: "Relative time span. Mutually exclusive with `start`/`end`. Allowed: `1m`, `5m`, `10m`, `15m`, `30m`, `1h`, `4h`, `1d`, `2d`, `1w`, `1mo`, `3mo`, `6mo`, `1y`, `alert`.",
						Optional:            true,
						Validators: []validator.String{
							stringvalidator.OneOf("1m", "5m", "10m", "15m", "30m", "1h", "4h", "1d", "2d", "1w", "1mo", "3mo", "6mo", "1y", "alert"),
						},
					},
					"start": schema.StringAttribute{
						MarkdownDescription: "Absolute start timestamp (RFC3339). Required when `end` is set.",
						Optional:            true,
					},
					"end": schema.StringAttribute{
						MarkdownDescription: "Absolute end timestamp (RFC3339). Required when `start` is set.",
						Optional:            true,
					},
				},
			},
		},
	}
}

// ConfigValidators returns plan-time validators for the resource config.
func (r *NotebookResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		notebookTimeValidator{},
	}
}

// notebookTimeValidator enforces mutual exclusivity and required-together rules on the time block.
type notebookTimeValidator struct{}

func (v notebookTimeValidator) Description(_ context.Context) string {
	return "time.live_span is mutually exclusive with time.start and time.end; time.start and time.end must be set together"
}

func (v notebookTimeValidator) MarkdownDescription(_ context.Context) string {
	return v.Description(context.Background())
}

func (v notebookTimeValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data NotebookResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.Time == nil {
		return
	}
	t := data.Time
	hasLive := !t.LiveSpan.IsNull() && !t.LiveSpan.IsUnknown()
	hasStart := !t.Start.IsNull() && !t.Start.IsUnknown()
	hasEnd := !t.End.IsNull() && !t.End.IsUnknown()

	if hasLive && hasStart {
		resp.Diagnostics.AddAttributeError(
			path.Root("time").AtName("live_span"),
			"Conflicting time attributes",
			"time.live_span conflicts with time.start",
		)
	}
	if hasLive && hasEnd {
		resp.Diagnostics.AddAttributeError(
			path.Root("time").AtName("live_span"),
			"Conflicting time attributes",
			"time.live_span conflicts with time.end",
		)
	}
	if hasStart != hasEnd {
		resp.Diagnostics.AddAttributeError(
			path.Root("time").AtName("start"),
			"Required together",
			"time.start and time.end must be specified together",
		)
	}
}

// Configure stores the DatadogClients passed from the provider.
func (r *NotebookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	clients, ok := req.ProviderData.(*DatadogClients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *DatadogClients, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.notebooksAPI = clients.NotebooksAPI
	r.apiCtx = clients.APIContext
}

// Create creates a new Datadog Notebook.
func (r *NotebookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data NotebookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cells, err := cellsFromJSON(data.Cells.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid cells JSON", err.Error())
		return
	}

	globalTime, err := buildGlobalTime(data.Time)
	if err != nil {
		resp.Diagnostics.AddError("Invalid time configuration", err.Error())
		return
	}

	attrs := datadogV1.NewNotebookCreateDataAttributes(cells, data.Name.ValueString(), globalTime)

	if metadata := buildMetadata(data); metadata != nil {
		attrs.SetMetadata(*metadata)
	}

	// Set template_variables via AdditionalProperties if present.
	if len(data.TemplateVariables) > 0 {
		tvs := buildTemplateVariables(ctx, data.TemplateVariables)
		attrs.AdditionalProperties = map[string]interface{}{
			"template_variables": tvs,
		}
	}

	createData := datadogV1.NewNotebookCreateData(*attrs, datadogV1.NOTEBOOKRESOURCETYPE_NOTEBOOKS)
	body := datadogV1.NewNotebookCreateRequest(*createData)

	notebookResp, httpResp, err := r.notebooksAPI.CreateNotebook(r.apiCtx, *body)
	if err != nil {
		detail := err.Error()
		if httpResp != nil {
			if apiErr, ok := err.(interface{ Body() []byte }); ok {
				detail = fmt.Sprintf("%s — API response: %s", err.Error(), apiErr.Body())
			}
		}
		resp.Diagnostics.AddError("Error creating notebook", detail)
		return
	}

	data.ID = types.StringValue(strconv.FormatInt(notebookResp.Data.GetId(), 10))
	mapResponseToModel(ctx, notebookResp.Data.GetAttributes(), &data)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read refreshes the Terraform state with the latest Datadog Notebook data.
func (r *NotebookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data NotebookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	notebookID, err := strconv.ParseInt(data.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid notebook ID", err.Error())
		return
	}

	notebookResp, httpResp, err := r.notebooksAPI.GetNotebook(r.apiCtx, notebookID)
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading notebook", err.Error())
		return
	}

	mapResponseToModel(ctx, notebookResp.Data.GetAttributes(), &data)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update updates an existing Datadog Notebook in-place.
func (r *NotebookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data NotebookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	notebookID, err := strconv.ParseInt(data.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid notebook ID", err.Error())
		return
	}

	cells, err := cellsToUpdateCells(data.Cells.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid cells JSON", err.Error())
		return
	}

	globalTime, err := buildGlobalTime(data.Time)
	if err != nil {
		resp.Diagnostics.AddError("Invalid time configuration", err.Error())
		return
	}

	attrs := datadogV1.NewNotebookUpdateDataAttributes(cells, data.Name.ValueString(), globalTime)

	if metadata := buildMetadata(data); metadata != nil {
		attrs.SetMetadata(*metadata)
	}

	if len(data.TemplateVariables) > 0 {
		tvs := buildTemplateVariables(ctx, data.TemplateVariables)
		attrs.AdditionalProperties = map[string]interface{}{
			"template_variables": tvs,
		}
	}

	updateData := datadogV1.NewNotebookUpdateData(*attrs, datadogV1.NOTEBOOKRESOURCETYPE_NOTEBOOKS)
	body := datadogV1.NewNotebookUpdateRequest(*updateData)

	notebookResp, _, err := r.notebooksAPI.UpdateNotebook(r.apiCtx, notebookID, *body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating notebook", err.Error())
		return
	}

	mapResponseToModel(ctx, notebookResp.Data.GetAttributes(), &data)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete deletes a Datadog Notebook.
func (r *NotebookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data NotebookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	notebookID, err := strconv.ParseInt(data.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid notebook ID", err.Error())
		return
	}

	httpResp, err := r.notebooksAPI.DeleteNotebook(r.apiCtx, notebookID)
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Error deleting notebook", err.Error())
		return
	}
}

// ImportState imports a Datadog Notebook by ID.
func (r *NotebookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// --- Helper functions ---

// cellsFromJSON unmarshals a JSON string into a slice of NotebookCellCreateRequest.
func cellsFromJSON(cellsJSON string) ([]datadogV1.NotebookCellCreateRequest, error) {
	var cells []datadogV1.NotebookCellCreateRequest
	if err := json.Unmarshal([]byte(cellsJSON), &cells); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cells JSON: %w", err)
	}
	return cells, nil
}

// cellsToUpdateCells converts a cells JSON string into NotebookUpdateCell slice (for Update).
func cellsToUpdateCells(cellsJSON string) ([]datadogV1.NotebookUpdateCell, error) {
	cells, err := cellsFromJSON(cellsJSON)
	if err != nil {
		return nil, err
	}
	updateCells := make([]datadogV1.NotebookUpdateCell, len(cells))
	for i, c := range cells {
		updateCells[i] = datadogV1.NotebookCellCreateRequestAsNotebookUpdateCell(&c)
	}
	return updateCells, nil
}

// cellsToJSON marshals response cells back into a JSON string for state storage,
// stripping the server-assigned cell `id` field so the stored JSON matches the
// user's original jsonencode([...]) input.
func cellsToJSON(cells []datadogV1.NotebookCellResponse) (string, error) {
	b, err := json.Marshal(cells)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cells to JSON: %w", err)
	}
	// Remove "id" from each cell object so state matches the plan format.
	var raw []map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return "", fmt.Errorf("failed to unmarshal cells for id-stripping: %w", err)
	}
	for _, cell := range raw {
		delete(cell, "id")
	}
	out, err := json.Marshal(raw)
	if err != nil {
		return "", fmt.Errorf("failed to re-marshal cells after id-stripping: %w", err)
	}
	return string(out), nil
}

// buildGlobalTime constructs a NotebookGlobalTime from the model.
func buildGlobalTime(t *NotebookTimeModel) (datadogV1.NotebookGlobalTime, error) {
	if t == nil {
		// Default to live 1h
		liveSpan := datadogV1.WIDGETLIVESPAN_PAST_ONE_HOUR
		return datadogV1.NotebookRelativeTimeAsNotebookGlobalTime(
			datadogV1.NewNotebookRelativeTime(liveSpan),
		), nil
	}
	if !t.LiveSpan.IsNull() && !t.LiveSpan.IsUnknown() {
		liveSpan := datadogV1.WidgetLiveSpan(t.LiveSpan.ValueString())
		return datadogV1.NotebookRelativeTimeAsNotebookGlobalTime(
			datadogV1.NewNotebookRelativeTime(liveSpan),
		), nil
	}
	if !t.Start.IsNull() && !t.End.IsNull() {
		start, err := time.Parse(time.RFC3339, t.Start.ValueString())
		if err != nil {
			return datadogV1.NotebookGlobalTime{}, fmt.Errorf("invalid time.start: %w", err)
		}
		end, err := time.Parse(time.RFC3339, t.End.ValueString())
		if err != nil {
			return datadogV1.NotebookGlobalTime{}, fmt.Errorf("invalid time.end: %w", err)
		}
		return datadogV1.NotebookAbsoluteTimeAsNotebookGlobalTime(
			datadogV1.NewNotebookAbsoluteTime(end, start),
		), nil
	}
	// Default to live 1h if time block is present but empty
	liveSpan := datadogV1.WIDGETLIVESPAN_PAST_ONE_HOUR
	return datadogV1.NotebookRelativeTimeAsNotebookGlobalTime(
		datadogV1.NewNotebookRelativeTime(liveSpan),
	), nil
}

// buildMetadata constructs NotebookMetadata from the resource model.
// Returns nil when no metadata fields are set, so the metadata key is omitted
// from the API request (Datadog requires at least one field when metadata is present).
func buildMetadata(data NotebookResourceModel) *datadogV1.NotebookMetadata {
	hasAny := false
	metadata := datadogV1.NotebookMetadata{}
	if !data.IsTemplate.IsNull() && !data.IsTemplate.IsUnknown() {
		v := data.IsTemplate.ValueBool()
		metadata.SetIsTemplate(v)
		hasAny = true
	}
	if !data.TakeSnapshots.IsNull() && !data.TakeSnapshots.IsUnknown() {
		v := data.TakeSnapshots.ValueBool()
		metadata.SetTakeSnapshots(v)
		hasAny = true
	}
	if !data.Type.IsNull() && !data.Type.IsUnknown() {
		t := datadogV1.NotebookMetadataType(data.Type.ValueString())
		metadata.SetType(t)
		hasAny = true
	}
	if !hasAny {
		return nil
	}
	return &metadata
}

// buildTemplateVariables converts the Terraform model to API-compatible slice.
func buildTemplateVariables(ctx context.Context, tvs []TemplateVariableModel) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(tvs))
	for _, tv := range tvs {
		m := map[string]interface{}{
			"name": tv.Name.ValueString(),
		}
		if !tv.Prefix.IsNull() {
			m["prefix"] = tv.Prefix.ValueString()
		}
		if !tv.Default.IsNull() {
			m["default"] = tv.Default.ValueString()
		}
		if !tv.AvailableValues.IsNull() {
			var vals []string
			tv.AvailableValues.ElementsAs(ctx, &vals, false) //nolint:errcheck
			m["available_values"] = vals
		}
		result = append(result, m)
	}
	return result
}

// mapResponseToModel maps a NotebookResponseDataAttributes back into the Terraform state model.
func mapResponseToModel(ctx context.Context, attrs datadogV1.NotebookResponseDataAttributes, data *NotebookResourceModel) {
	data.Name = types.StringValue(attrs.GetName())

	cellsJSON, err := cellsToJSON(attrs.GetCells())
	if err != nil {
		return
	}
	data.Cells = types.StringValue(cellsJSON)

	// Map metadata.
	if meta, ok := attrs.GetMetadataOk(); ok && meta != nil {
		if !meta.Type.IsSet() || meta.Type.Get() == nil {
			data.Type = types.StringNull()
		} else {
			data.Type = types.StringValue(string(meta.GetType()))
		}
		// Only propagate is_template/take_snapshots when explicitly true, to
		// avoid replacing a null plan value with the API default of false.
		if meta.IsTemplate != nil && *meta.IsTemplate {
			data.IsTemplate = types.BoolValue(true)
		}
		if meta.TakeSnapshots != nil && *meta.TakeSnapshots {
			data.TakeSnapshots = types.BoolValue(true)
		}
	}

	// Map time.
	globalTime := attrs.GetTime()
	if globalTime.NotebookRelativeTime != nil {
		data.Time = &NotebookTimeModel{
			LiveSpan: types.StringValue(string(globalTime.NotebookRelativeTime.LiveSpan)),
			Start:    types.StringNull(),
			End:      types.StringNull(),
		}
	} else if globalTime.NotebookAbsoluteTime != nil {
		data.Time = &NotebookTimeModel{
			LiveSpan: types.StringNull(),
			Start:    types.StringValue(globalTime.NotebookAbsoluteTime.Start.Format(time.RFC3339)),
			End:      types.StringValue(globalTime.NotebookAbsoluteTime.End.Format(time.RFC3339)),
		}
	}

	// Map template_variables from AdditionalProperties; ignore empty list so a
	// null plan value is not replaced with an empty list.
	if tvRaw, ok := attrs.AdditionalProperties["template_variables"]; ok && tvRaw != nil {
		tvs := parseTemplateVariables(ctx, tvRaw)
		if len(tvs) > 0 {
			data.TemplateVariables = tvs
		}
	}
}

// parseTemplateVariables attempts to parse template variables from AdditionalProperties.
func parseTemplateVariables(ctx context.Context, raw interface{}) []TemplateVariableModel {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var items []map[string]interface{}
	if err := json.Unmarshal(b, &items); err != nil {
		return nil
	}
	result := make([]TemplateVariableModel, 0, len(items))
	for _, item := range items {
		tv := TemplateVariableModel{}
		if v, ok := item["name"].(string); ok {
			tv.Name = types.StringValue(v)
		}
		if v, ok := item["prefix"].(string); ok {
			tv.Prefix = types.StringValue(v)
		} else {
			tv.Prefix = types.StringNull()
		}
		if v, ok := item["default"].(string); ok {
			tv.Default = types.StringValue(v)
		} else {
			tv.Default = types.StringNull()
		}
		if v, ok := item["available_values"].([]interface{}); ok {
			strs := make([]string, 0, len(v))
			for _, s := range v {
				if sv, ok := s.(string); ok {
					strs = append(strs, sv)
				}
			}
			elems, _ := types.ListValueFrom(ctx, types.StringType, strs)
			tv.AvailableValues = elems
		} else {
			tv.AvailableValues = types.ListNull(types.StringType)
		}
		result = append(result, tv)
	}
	return result
}
