package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	datadogV1 "github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
				MarkdownDescription: "Bare team names (e.g. `sre`). The provider translates these to `team:<name>` tags in Datadog. Maximum 5 entries.",
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

	planCellsJSON := data.Cells.ValueString()
	cells, err := cellsFromJSON(planCellsJSON)
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

	// Init AdditionalProperties once and set template_variables and teams tags.
	if attrs.AdditionalProperties == nil {
		attrs.AdditionalProperties = make(map[string]interface{})
	}
	if len(data.TemplateVariables) > 0 {
		tvs, d := buildTemplateVariables(ctx, data.TemplateVariables)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		attrs.AdditionalProperties["template_variables"] = tvs
	}
	tags, d := teamsToTagSlice(ctx, data.Teams)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(tags) > 0 {
		attrs.AdditionalProperties["tags"] = tags
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
	resp.Diagnostics.Append(mapResponseToModel(ctx, notebookResp.Data.GetAttributes(), &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Use plan cells directly: the API accepted the request so the notebook now contains
	// what we sent. The API response may inject separator cells or reorder content, so
	// we trust the plan as source of truth for the cells state.
	data.Cells = types.StringValue(planCellsJSON)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read refreshes the Terraform state with the latest Datadog Notebook data.
func (r *NotebookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data NotebookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	priorCellsJSON := data.Cells.ValueString()

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

	resp.Diagnostics.Append(mapResponseToModel(ctx, notebookResp.Data.GetAttributes(), &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Reconcile API response cells with the prior state (which mirrors the user's plan).
	// The Datadog API may inject separator cells or reorder content, causing cell-count
	// mismatches. When counts differ, keep the prior state so no spurious diff is shown.
	apiCellsJSON := data.Cells.ValueString()
	var apiCells, priorCells []map[string]interface{}
	if json.Unmarshal([]byte(apiCellsJSON), &apiCells) == nil &&
		json.Unmarshal([]byte(priorCellsJSON), &priorCells) == nil &&
		len(apiCells) == len(priorCells) {
		data.Cells = types.StringValue(preservePassthroughFields(apiCellsJSON, priorCellsJSON))
	} else {
		data.Cells = types.StringValue(priorCellsJSON)
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

	planCellsJSON := data.Cells.ValueString()
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

	// Init AdditionalProperties once and set template_variables and teams tags.
	if attrs.AdditionalProperties == nil {
		attrs.AdditionalProperties = make(map[string]interface{})
	}
	if len(data.TemplateVariables) > 0 {
		tvs, d := buildTemplateVariables(ctx, data.TemplateVariables)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		attrs.AdditionalProperties["template_variables"] = tvs
	}
	tags, d := teamsToTagSlice(ctx, data.Teams)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(tags) > 0 {
		attrs.AdditionalProperties["tags"] = tags
	}

	updateData := datadogV1.NewNotebookUpdateData(*attrs, datadogV1.NOTEBOOKRESOURCETYPE_NOTEBOOKS)
	body := datadogV1.NewNotebookUpdateRequest(*updateData)

	notebookResp, _, err := r.notebooksAPI.UpdateNotebook(r.apiCtx, notebookID, *body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating notebook", err.Error())
		return
	}

	resp.Diagnostics.Append(mapResponseToModel(ctx, notebookResp.Data.GetAttributes(), &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Use plan cells directly: same reasoning as Create — the API accepted the request
	// so the notebook now contains what we sent.
	data.Cells = types.StringValue(planCellsJSON)

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

// --- Helper functions ---

// cellAttrFields are cell-level fields that belong inside `attributes` in the SDK
// but are written at the top cell level by users (following the Datadog UI convention).
var cellAttrFields = []string{"graph_size", "split_by", "time"}

// graphSizeSupportedTypes lists definition.type values whose SDK attributes struct
// has a GraphSize field and whose API will return graph_size in responses.
var graphSizeSupportedTypes = map[string]bool{
	"timeseries":   true,
	"toplist":      true,
	"heatmap":      true,
	"distribution": true,
	"log_stream":   true,
}

// splitBySupportedTypes lists definition.type values whose SDK attributes struct
// has a SplitBy field and whose API will return split_by in responses.
var splitBySupportedTypes = map[string]bool{
	"timeseries":   true,
	"toplist":      true,
	"heatmap":      true,
	"distribution": true,
}

// cellDefinitionType returns the definition.type string for a raw cell map.
func cellDefinitionType(cell map[string]interface{}) string {
	attrs, _ := cell["attributes"].(map[string]interface{})
	def, _ := attrs["definition"].(map[string]interface{})
	t, _ := def["type"].(string)
	return t
}

// normalizeCellsForAPI moves graph_size, split_by, and time from the top cell level
// into attributes before sending to the Datadog API (where they belong in the SDK types).
// It also strips graph_size and split_by from cell types that don't support them.
func normalizeCellsForAPI(raw []map[string]interface{}) {
	for _, cell := range raw {
		attrs, ok := cell["attributes"].(map[string]interface{})
		if !ok {
			continue
		}
		defType := cellDefinitionType(cell)
		for _, field := range cellAttrFields {
			if val, exists := cell[field]; exists {
				if _, inAttrs := attrs[field]; !inAttrs {
					attrs[field] = val
				}
				delete(cell, field)
			}
		}
		// Strip fields from attributes when the cell type doesn't support them,
		// so we don't send unsupported fields to the API.
		if !graphSizeSupportedTypes[defType] {
			delete(attrs, "graph_size")
		}
		if !splitBySupportedTypes[defType] {
			delete(attrs, "split_by")
		}
	}
}

// preservePassthroughFields copies graph_size and split_by from planCells into stateCells
// for cell types where the Datadog API silently ignores these fields and never returns them.
// This prevents a false "Provider produced inconsistent result" error when the plan has a
// field that the API echoes back but the state doesn't.
// Cells are matched positionally; if counts differ the original stateCellsJSON is returned unchanged.
func preservePassthroughFields(stateCellsJSON, planCellsJSON string) string {
	var stateCells, planCells []map[string]interface{}
	if err := json.Unmarshal([]byte(stateCellsJSON), &stateCells); err != nil {
		return stateCellsJSON
	}
	if err := json.Unmarshal([]byte(planCellsJSON), &planCells); err != nil {
		return stateCellsJSON
	}
	if len(stateCells) != len(planCells) {
		return stateCellsJSON
	}
	for i, stateCell := range stateCells {
		planCell := planCells[i]
		defType := cellDefinitionType(stateCell)
		stateAttrs, _ := stateCell["attributes"].(map[string]interface{})
		planAttrs, _ := planCell["attributes"].(map[string]interface{})
		if stateAttrs == nil || planAttrs == nil {
			continue
		}
		if !graphSizeSupportedTypes[defType] {
			if val, ok := planAttrs["graph_size"]; ok {
				stateAttrs["graph_size"] = val
			}
		}
		if !splitBySupportedTypes[defType] {
			if val, ok := planAttrs["split_by"]; ok {
				stateAttrs["split_by"] = val
			}
		}
		// For cell types that the Datadog API doesn't round-trip the per-cell `time`
		// override for (e.g. treemap, sunburst, manage_status), preserve it from the plan.
		// graphSizeSupportedTypes is a reliable proxy for "graph cell types that the
		// Notebooks API fully supports" (timeseries, toplist, heatmap, distribution, log_stream).
		if !graphSizeSupportedTypes[defType] {
			if val, ok := planAttrs["time"]; ok {
				stateAttrs["time"] = val
			} else {
				delete(stateAttrs, "time")
			}
		}
		// The Datadog API normalizes markdown text (adds blank lines between list items,
		// escapes tildes, etc.) before storing it. Preserve the plan/prior-state text so
		// the state always matches what the user wrote, preventing perpetual diffs.
		if defType == "markdown" {
			stateDef, _ := stateAttrs["definition"].(map[string]interface{})
			planDef, _ := planAttrs["definition"].(map[string]interface{})
			if stateDef != nil && planDef != nil {
				if text, ok := planDef["text"]; ok {
					stateDef["text"] = text
				}
			}
		}
	}
	out, err := json.Marshal(stateCells)
	if err != nil {
		return stateCellsJSON
	}
	return string(out)
}

// cellsFromJSON unmarshals a JSON string into a slice of NotebookCellCreateRequest.
func cellsFromJSON(cellsJSON string) ([]datadogV1.NotebookCellCreateRequest, error) {
	var rawCells []map[string]interface{}
	if err := json.Unmarshal([]byte(cellsJSON), &rawCells); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cells JSON: %w", err)
	}
	normalizeCellsForAPI(rawCells)
	normalized, err := json.Marshal(rawCells)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal normalized cells: %w", err)
	}
	var cells []datadogV1.NotebookCellCreateRequest
	if err := json.Unmarshal(normalized, &cells); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cells into SDK types: %w", err)
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
// stripping the server-assigned cell `id`, empty markdown separator cells inserted
// by the Datadog API, and null-valued attribute keys to keep state consistent with
// the user's original jsonencode([...]) input.
func cellsToJSON(cells []datadogV1.NotebookCellResponse) (string, error) {
	b, err := json.Marshal(cells)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cells to JSON: %w", err)
	}
	var raw []map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return "", fmt.Errorf("failed to unmarshal cells for id-stripping: %w", err)
	}

	// Filter out empty markdown separator cells injected by the Datadog API between
	// adjacent non-markdown cells. These have definition.text == "" and are never
	// written by users, so they must not appear in state.
	filtered := raw[:0]
	for _, cell := range raw {
		if !isEmptyMarkdownSeparator(cell) {
			filtered = append(filtered, cell)
		}
	}
	raw = filtered

	for _, cell := range raw {
		delete(cell, "id")
		// Strip null-valued keys from attributes to avoid spurious diffs when the
		// API returns e.g. "time": null for cells that have no per-cell time override.
		if attrs, ok := cell["attributes"].(map[string]interface{}); ok {
			for k, v := range attrs {
				if v == nil {
					delete(attrs, k)
				}
			}
		}
	}
	out, err := json.Marshal(raw)
	if err != nil {
		return "", fmt.Errorf("failed to re-marshal cells after id-stripping: %w", err)
	}
	return string(out), nil
}

// isEmptyMarkdownSeparator returns true for empty markdown cells (definition.text == "")
// that the Datadog API inserts between adjacent non-markdown cells.
func isEmptyMarkdownSeparator(cell map[string]interface{}) bool {
	attrs, _ := cell["attributes"].(map[string]interface{})
	if attrs == nil {
		return false
	}
	def, _ := attrs["definition"].(map[string]interface{})
	if def == nil {
		return false
	}
	t, _ := def["type"].(string)
	text, _ := def["text"].(string)
	return t == "markdown" && text == ""
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
// Returns diagnostics for any errors encountered during ElementsAs conversion.
func buildTemplateVariables(ctx context.Context, tvs []TemplateVariableModel) ([]map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
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
			d := tv.AvailableValues.ElementsAs(ctx, &vals, false)
			diags.Append(d...)
			if d.HasError() {
				return nil, diags
			}
			m["available_values"] = vals
		}
		result = append(result, m)
	}
	return result, diags
}

// mapResponseToModel maps a NotebookResponseDataAttributes back into the Terraform state model.
// Returns diagnostics for any errors encountered during cell serialization or type conversion.
func mapResponseToModel(ctx context.Context, attrs datadogV1.NotebookResponseDataAttributes, data *NotebookResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.Name = types.StringValue(attrs.GetName())

	cellsJSON, err := cellsToJSON(attrs.GetCells())
	if err != nil {
		diags.AddError("Error serializing notebook cells", err.Error())
		return diags
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
		tvs, d := parseTemplateVariables(ctx, tvRaw)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		if len(tvs) > 0 {
			data.TemplateVariables = tvs
		}
	}

	// Map teams from AdditionalProperties["tags"].
	if tagsRaw, ok := attrs.AdditionalProperties["tags"]; ok && tagsRaw != nil {
		names := tagsToTeamNames(tagsRaw)
		if len(names) > 0 {
			teams, d := types.ListValueFrom(ctx, types.StringType, names)
			diags.Append(d...)
			if diags.HasError() {
				return diags
			}
			data.Teams = teams
		}
	}

	return diags
}

// parseTemplateVariables attempts to parse template variables from AdditionalProperties.
// Returns diagnostics for any errors encountered during ListValueFrom conversion.
func parseTemplateVariables(ctx context.Context, raw interface{}) ([]TemplateVariableModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, diags
	}
	var items []map[string]interface{}
	if err := json.Unmarshal(b, &items); err != nil {
		return nil, diags
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
		if v, ok := item["available_values"].([]interface{}); ok && len(v) > 0 {
			strs := make([]string, 0, len(v))
			for _, s := range v {
				if sv, ok := s.(string); ok {
					strs = append(strs, sv)
				}
			}
			elems, d := types.ListValueFrom(ctx, types.StringType, strs)
			diags.Append(d...)
			if d.HasError() {
				return nil, diags
			}
			tv.AvailableValues = elems
		} else {
			// Treat empty or absent available_values from the API as null so state
			// matches configs that omit the field (avoids spurious [] → null diffs).
			tv.AvailableValues = types.ListNull(types.StringType)
		}
		result = append(result, tv)
	}
	return result, diags
}

// teamsToTagSlice converts bare team names (e.g. ["sre"]) into "team:<name>" tag strings.
// Returns nil when the list is null, unknown, or empty.
// Returns diagnostics for any errors encountered during ElementsAs conversion.
func teamsToTagSlice(ctx context.Context, teams types.List) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if teams.IsNull() || teams.IsUnknown() {
		return nil, diags
	}
	var names []string
	d := teams.ElementsAs(ctx, &names, false)
	diags.Append(d...)
	if d.HasError() {
		return nil, diags
	}
	if len(names) == 0 {
		return nil, diags
	}
	tags := make([]string, len(names))
	for i, n := range names {
		tags[i] = "team:" + n
	}
	return tags, diags
}

// tagsToTeamNames filters a raw AdditionalProperties tags value for "team:"-prefixed entries
// and returns the bare team names. Returns nil when no team tags are present.
func tagsToTeamNames(raw interface{}) []string {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var all []string
	if err := json.Unmarshal(b, &all); err != nil {
		return nil
	}
	var teams []string
	for _, t := range all {
		if strings.HasPrefix(t, "team:") {
			teams = append(teams, strings.TrimPrefix(t, "team:"))
		}
	}
	return teams
}