package restapitoolintegration

import (
	"backend/api/msgmate/tools"
	"backend/database"
	"backend/server/util"
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/msgmate-io/go-integration-interface/integrationinterface"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

type DynamicRESTToolUpsertRequest struct {
	Name                           string                   `json:"name"`
	FunctionName                   string                   `json:"function_name"`
	Description                    string                   `json:"description"`
	AdminOnly                      bool                     `json:"admin_only"`
	RequiresConfirmation           bool                     `json:"requires_confirmation"`
	StopOnFirstConfirmableToolCall bool                     `json:"stop_on_first_confirmable_tool_call"`
	ConfirmationBlockMessage       string                   `json:"confirmation_block_message"`
	Enabled                        *bool                    `json:"enabled,omitempty"`
	OpenAPISourceType              string                   `json:"openapi_source_type"`
	OpenAPISource                  string                   `json:"openapi_source"`
	OperationID                    string                   `json:"operation_id,omitempty"`
	HTTPMethod                     string                   `json:"http_method,omitempty"`
	Path                           string                   `json:"path,omitempty"`
	BaseURLSource                  string                   `json:"base_url_source,omitempty"`
	BaseURLInputName               string                   `json:"base_url_input_name,omitempty"`
	ParamBindings                  []map[string]interface{} `json:"param_bindings,omitempty"`
	SafetyPolicy                   map[string]interface{}   `json:"safety_policy,omitempty"`
}

type dynamicRESTToolListRow struct {
	UUID                           string                   `json:"uuid"`
	Name                           string                   `json:"name"`
	FunctionName                   string                   `json:"function_name"`
	Description                    string                   `json:"description"`
	AdminOnly                      bool                     `json:"admin_only"`
	RequiresConfirmation           bool                     `json:"requires_confirmation"`
	StopOnFirstConfirmableToolCall bool                     `json:"stop_on_first_confirmable_tool_call"`
	ConfirmationBlockMessage       string                   `json:"confirmation_block_message"`
	Enabled                        bool                     `json:"enabled"`
	OpenAPISourceType              string                   `json:"openapi_source_type"`
	OperationID                    string                   `json:"operation_id"`
	HTTPMethod                     string                   `json:"http_method"`
	Path                           string                   `json:"path"`
	BaseURLSource                  string                   `json:"base_url_source"`
	BaseURLInputName               string                   `json:"base_url_input_name"`
	ParamBindings                  []map[string]interface{} `json:"param_bindings"`
	SafetyPolicy                   map[string]interface{}   `json:"safety_policy"`
	CreatedAtUnix                  int64                    `json:"created_at_unix"`
	UpdatedAtUnix                  int64                    `json:"updated_at_unix"`
}

type dynamicRESTToolDetailResponse struct {
	Row        dynamicRESTToolListRow `json:"row"`
	CallSchema map[string]interface{} `json:"call_schema"`
	InitSchema map[string]interface{} `json:"init_schema"`
}

type restToolParamBinding struct {
	InputName   string `json:"input_name"`
	Source      string `json:"source"`
	In          string `json:"in"`
	Name        string `json:"name"`
	Required    *bool  `json:"required,omitempty"`
	Description string `json:"description,omitempty"`
}

type restToolSafetyPolicy struct {
	AllowHosts          []string `json:"allow_hosts,omitempty"`
	AllowPrivateIPs     bool     `json:"allow_private_ips,omitempty"`
	TimeoutSeconds      int      `json:"timeout_seconds,omitempty"`
	MaxResponseBody     int64    `json:"max_response_body_bytes,omitempty"`
	ResponseCensorPaths []string `json:"response_censor_paths,omitempty"`
}

type operationParam struct {
	In          string
	Name        string
	Required    bool
	Description string
	Schema      map[string]interface{}
}

type compiledBinding struct {
	restToolParamBinding
	Required bool
}

type dynamicRESTToolSpec struct {
	Method           string
	Path             string
	BaseURL          string
	BaseURLSource    string
	BaseURLInputName string
	Bindings         []compiledBinding
	Safety           restToolSafetyPolicy
}

var restAPIToolRoutes = []string{
	"GET /api/v1/integrations/rest_api_tool/tools",
	"GET /api/v1/integrations/rest_api_tool/tools/{tool_uuid}",
	"POST /api/v1/integrations/rest_api_tool/tools",
	"PUT /api/v1/integrations/rest_api_tool/tools/{tool_name}",
	"DELETE /api/v1/integrations/rest_api_tool/tools/{tool_name}",
}

//go:embed frontend_assets
var restAPIToolFrontendAssets embed.FS

//go:embed README.md
var restAPIToolReadmeMarkdown string

var restAPIToolFrontendPages = []integrationinterface.FrontendPage{
	{
		Route:       "/integrations/rest_api_tool/tools",
		Public:      false,
		Description: "List registered REST API tools for the current user.",
		AssetPath:   "tools/index.html",
	},
	{
		Route:       "/integrations/rest_api_tool/tools/{tool_uuid}",
		Public:      false,
		Description: "Show details for one registered REST API tool.",
		AssetPath:   "tools/tool_uuid/index.html",
	},
}

func init() {
	integrationinterface.MustRegister(integrationinterface.Definition{
		Name:           "rest_api_tool",
		ReadmeMarkdown: strings.TrimSpace(restAPIToolReadmeMarkdown),
		APIRoutes:      append([]string(nil), restAPIToolRoutes...),
		FrontendPages:  append([]integrationinterface.FrontendPage(nil), restAPIToolFrontendPages...),
		FrontendAssets: mustSubFS(restAPIToolFrontendAssets, "frontend_assets"),
		ModelProviders: []func() []interface{}{
			func() []interface{} {
				return []interface{}{&database.DynamicRESTTool{}}
			},
		},
		RouteRegistrar: registerRoutes,
	})
}

func mustSubFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

func registerRoutes(v1Private *http.ServeMux, _ *http.ServeMux) {
	v1Private.HandleFunc(v1PrivatePattern(restAPIToolRoutes[0]), listDynamicRESTTools)
	v1Private.HandleFunc(v1PrivatePattern(restAPIToolRoutes[1]), getDynamicRESTToolByUUID)
	v1Private.HandleFunc(v1PrivatePattern(restAPIToolRoutes[2]), createDynamicRESTTool)
	v1Private.HandleFunc(v1PrivatePattern(restAPIToolRoutes[3]), updateDynamicRESTTool)
	v1Private.HandleFunc(v1PrivatePattern(restAPIToolRoutes[4]), deleteDynamicRESTTool)
}

func v1PrivatePattern(fullRoute string) string {
	const v1Prefix = "/api/v1"
	idx := strings.Index(fullRoute, " ")
	if idx < 0 || idx+1 >= len(fullRoute) {
		return fullRoute
	}
	method := strings.TrimSpace(fullRoute[:idx])
	path := strings.TrimSpace(fullRoute[idx+1:])
	if strings.HasPrefix(path, v1Prefix) {
		path = strings.TrimPrefix(path, v1Prefix)
		if path == "" {
			path = "/"
		}
	}
	return method + " " + path
}

// @Summary      List REST API tools
// @Description  List owner-scoped runtime REST API tools.
// @Tags         integrations
// @Produce      json
// @Security     SessionAuth
// @Success      200 {object} map[string]interface{}
// @Router       /api/v1/integrations/rest_api_tool/tools [get]
func listDynamicRESTTools(w http.ResponseWriter, r *http.Request) {
	DB, user, err := util.GetDBAndUser(r)
	if err != nil {
		http.Error(w, "Unable to get database or user", http.StatusBadRequest)
		return
	}
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var rows []database.DynamicRESTTool
	if err := DB.Where("owner_user_id = ?", user.ID).Order("name asc").Find(&rows).Error; err != nil {
		http.Error(w, "Failed to list dynamic tools", http.StatusInternalServerError)
		return
	}
	items := make([]dynamicRESTToolListRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, buildDynamicRESTToolListRow(row))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"rows": items})
}

// @Summary      Get REST API tool
// @Description  Get one owner-scoped runtime REST API tool by UUID.
// @Tags         integrations
// @Produce      json
// @Security     SessionAuth
// @Param        tool_uuid path string true "Tool UUID"
// @Success      200 {object} dynamicRESTToolDetailResponse
// @Router       /api/v1/integrations/rest_api_tool/tools/{tool_uuid} [get]
func getDynamicRESTToolByUUID(w http.ResponseWriter, r *http.Request) {
	DB, user, err := util.GetDBAndUser(r)
	if err != nil {
		http.Error(w, "Unable to get database or user", http.StatusBadRequest)
		return
	}
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	toolUUID := strings.TrimSpace(r.PathValue("tool_uuid"))
	if toolUUID == "" {
		http.Error(w, "tool_uuid is required", http.StatusBadRequest)
		return
	}

	var row database.DynamicRESTTool
	if err := DB.Where("owner_user_id = ? AND uuid = ?", user.ID, toolUUID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "tool not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load dynamic rest tool", http.StatusInternalServerError)
		return
	}

	def, err := BuildDynamicRESTToolDefinition(row)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid dynamic rest tool definition: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dynamicRESTToolDetailResponse{
		Row:        buildDynamicRESTToolListRow(row),
		CallSchema: def.InputSchema,
		InitSchema: def.InitSchema,
	})
}

// @Summary      Create REST API tool
// @Description  Create an owner-scoped runtime REST API tool definition.
// @Tags         integrations
// @Accept       json
// @Produce      json
// @Security     SessionAuth
// @Param        body body DynamicRESTToolUpsertRequest true "REST API tool definition"
// @Success      200 {object} map[string]interface{}
// @Router       /api/v1/integrations/rest_api_tool/tools [post]
func createDynamicRESTTool(w http.ResponseWriter, r *http.Request) {
	DB, user, err := util.GetDBAndUser(r)
	if err != nil {
		http.Error(w, "Unable to get database or user", http.StatusBadRequest)
		return
	}
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req DynamicRESTToolUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	row, err := buildDynamicRESTToolRowFromRequest(user, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err := BuildDynamicRESTToolDefinition(row); err != nil {
		http.Error(w, fmt.Sprintf("invalid dynamic rest tool definition: %v", err), http.StatusBadRequest)
		return
	}

	var existing database.DynamicRESTTool
	lookupErr := DB.Unscoped().
		Where("owner_user_id = ? AND name = ?", user.ID, row.Name).
		First(&existing).Error
	if lookupErr == nil {
		if existing.DeletedAt.Valid {
			applyDynamicRESTToolRow(&existing, row)
			existing.DeletedAt = gorm.DeletedAt{}
			if err := DB.Unscoped().Save(&existing).Error; err != nil {
				http.Error(w, "failed to restore dynamic rest tool", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "name": existing.Name})
			return
		}
		http.Error(w, "tool already exists", http.StatusConflict)
		return
	}
	if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		http.Error(w, "failed to check existing dynamic rest tool", http.StatusInternalServerError)
		return
	}

	if err := DB.Create(&row).Error; err != nil {
		http.Error(w, "failed to create dynamic rest tool", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "name": row.Name})
}

// @Summary      Update REST API tool
// @Description  Update an owner-scoped runtime REST API tool definition.
// @Tags         integrations
// @Accept       json
// @Produce      json
// @Security     SessionAuth
// @Param        tool_name path string true "Tool Name"
// @Success      200 {object} map[string]interface{}
// @Router       /api/v1/integrations/rest_api_tool/tools/{tool_name} [put]
func updateDynamicRESTTool(w http.ResponseWriter, r *http.Request) {
	DB, user, err := util.GetDBAndUser(r)
	if err != nil {
		http.Error(w, "Unable to get database or user", http.StatusBadRequest)
		return
	}
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	toolName := strings.TrimSpace(r.PathValue("tool_name"))
	if toolName == "" {
		http.Error(w, "tool_name is required", http.StatusBadRequest)
		return
	}

	var req DynamicRESTToolUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = toolName
	}

	row, err := buildDynamicRESTToolRowFromRequest(user, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err := BuildDynamicRESTToolDefinition(row); err != nil {
		http.Error(w, fmt.Sprintf("invalid dynamic rest tool definition: %v", err), http.StatusBadRequest)
		return
	}

	var existing database.DynamicRESTTool
	if err := DB.Where("owner_user_id = ? AND name = ?", user.ID, toolName).First(&existing).Error; err != nil {
		http.Error(w, "tool not found", http.StatusNotFound)
		return
	}

	applyDynamicRESTToolRow(&existing, row)
	if err := DB.Save(&existing).Error; err != nil {
		http.Error(w, "failed to update dynamic rest tool", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "name": existing.Name})
}

// @Summary      Delete REST API tool
// @Description  Delete an owner-scoped runtime REST API tool definition.
// @Tags         integrations
// @Produce      json
// @Security     SessionAuth
// @Param        tool_name path string true "Tool Name"
// @Success      200 {object} map[string]interface{}
// @Router       /api/v1/integrations/rest_api_tool/tools/{tool_name} [delete]
func deleteDynamicRESTTool(w http.ResponseWriter, r *http.Request) {
	DB, user, err := util.GetDBAndUser(r)
	if err != nil {
		http.Error(w, "Unable to get database or user", http.StatusBadRequest)
		return
	}
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	toolName := strings.TrimSpace(r.PathValue("tool_name"))
	if toolName == "" {
		http.Error(w, "tool_name is required", http.StatusBadRequest)
		return
	}
	if err := DB.Unscoped().Where("owner_user_id = ? AND name = ?", user.ID, toolName).Delete(&database.DynamicRESTTool{}).Error; err != nil {
		http.Error(w, "failed to delete dynamic rest tool", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func buildDynamicRESTToolRowFromRequest(user *database.User, req DynamicRESTToolUpsertRequest) (database.DynamicRESTTool, error) {
	if user == nil {
		return database.DynamicRESTTool{}, fmt.Errorf("user is required")
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return database.DynamicRESTTool{}, fmt.Errorf("name is required")
	}

	bindJSON, err := json.Marshal(req.ParamBindings)
	if err != nil {
		return database.DynamicRESTTool{}, fmt.Errorf("invalid param_bindings")
	}
	safetyJSON, err := json.Marshal(req.SafetyPolicy)
	if err != nil {
		return database.DynamicRESTTool{}, fmt.Errorf("invalid safety_policy")
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	return database.DynamicRESTTool{
		OwnerUserId:                    user.ID,
		Name:                           req.Name,
		FunctionName:                   req.FunctionName,
		Description:                    req.Description,
		AdminOnly:                      req.AdminOnly,
		RequiresConfirmation:           req.RequiresConfirmation,
		StopOnFirstConfirmableToolCall: req.StopOnFirstConfirmableToolCall,
		ConfirmationBlockMessage:       req.ConfirmationBlockMessage,
		Enabled:                        enabled,
		OpenAPISourceType:              req.OpenAPISourceType,
		OpenAPISource:                  req.OpenAPISource,
		OperationID:                    req.OperationID,
		HTTPMethod:                     req.HTTPMethod,
		Path:                           req.Path,
		BaseURLSource:                  req.BaseURLSource,
		BaseURLInputName:               req.BaseURLInputName,
		ParamBindings:                  bindJSON,
		SafetyPolicy:                   safetyJSON,
	}, nil
}

func applyDynamicRESTToolRow(target *database.DynamicRESTTool, row database.DynamicRESTTool) {
	target.OwnerUserId = row.OwnerUserId
	target.Name = row.Name
	target.FunctionName = row.FunctionName
	target.Description = row.Description
	target.AdminOnly = row.AdminOnly
	target.RequiresConfirmation = row.RequiresConfirmation
	target.StopOnFirstConfirmableToolCall = row.StopOnFirstConfirmableToolCall
	target.ConfirmationBlockMessage = row.ConfirmationBlockMessage
	target.Enabled = row.Enabled
	target.OpenAPISourceType = row.OpenAPISourceType
	target.OpenAPISource = row.OpenAPISource
	target.OperationID = row.OperationID
	target.HTTPMethod = row.HTTPMethod
	target.Path = row.Path
	target.BaseURLSource = row.BaseURLSource
	target.BaseURLInputName = row.BaseURLInputName
	target.ParamBindings = row.ParamBindings
	target.SafetyPolicy = row.SafetyPolicy
}

func buildDynamicRESTToolListRow(row database.DynamicRESTTool) dynamicRESTToolListRow {
	return dynamicRESTToolListRow{
		UUID:                           row.UUID,
		Name:                           row.Name,
		FunctionName:                   row.FunctionName,
		Description:                    row.Description,
		AdminOnly:                      row.AdminOnly,
		RequiresConfirmation:           row.RequiresConfirmation,
		StopOnFirstConfirmableToolCall: row.StopOnFirstConfirmableToolCall,
		ConfirmationBlockMessage:       row.ConfirmationBlockMessage,
		Enabled:                        row.Enabled,
		OpenAPISourceType:              row.OpenAPISourceType,
		OperationID:                    row.OperationID,
		HTTPMethod:                     row.HTTPMethod,
		Path:                           row.Path,
		BaseURLSource:                  row.BaseURLSource,
		BaseURLInputName:               row.BaseURLInputName,
		ParamBindings:                  parseJSONArrayOrEmpty(row.ParamBindings),
		SafetyPolicy:                   parseJSONObjectOrEmpty(row.SafetyPolicy),
		CreatedAtUnix:                  row.CreatedAt.Unix(),
		UpdatedAtUnix:                  row.UpdatedAt.Unix(),
	}
}

func ResolveUserDynamicRESTToolByName(db *gorm.DB, ownerUserID uint, toolName string) (*database.DynamicRESTTool, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}
	if ownerUserID == 0 {
		return nil, fmt.Errorf("owner user id is required")
	}
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return nil, fmt.Errorf("tool name is required")
	}

	var row database.DynamicRESTTool
	if err := db.Where("owner_user_id = ? AND name = ? AND enabled = ?", ownerUserID, toolName, true).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func BuildDynamicRESTToolSnapshot(row database.DynamicRESTTool) map[string]interface{} {
	return map[string]interface{}{
		"name":                                row.Name,
		"function_name":                       row.FunctionName,
		"description":                         row.Description,
		"admin_only":                          row.AdminOnly,
		"requires_confirmation":               row.RequiresConfirmation,
		"stop_on_first_confirmable_tool_call": row.StopOnFirstConfirmableToolCall,
		"confirmation_block_message":          row.ConfirmationBlockMessage,
		"openapi_source_type":                 row.OpenAPISourceType,
		"openapi_source":                      row.OpenAPISource,
		"operation_id":                        row.OperationID,
		"http_method":                         row.HTTPMethod,
		"path":                                row.Path,
		"base_url_source":                     row.BaseURLSource,
		"base_url_input_name":                 row.BaseURLInputName,
		"param_bindings":                      parseJSONArrayOrEmpty(row.ParamBindings),
		"safety_policy":                       parseJSONObjectOrEmpty(row.SafetyPolicy),
	}
}

func NewDynamicRESTToolDefinitionFromSnapshot(toolName string, dynamicToolsRaw interface{}) (tools.ToolDefinition, bool, error) {
	dynamicToolsMap, ok := dynamicToolsRaw.(map[string]interface{})
	if !ok || dynamicToolsMap == nil {
		return tools.ToolDefinition{}, false, nil
	}
	rawDef, exists := dynamicToolsMap[toolName]
	if !exists {
		return tools.ToolDefinition{}, false, nil
	}
	defMap, ok := rawDef.(map[string]interface{})
	if !ok {
		return tools.ToolDefinition{}, false, fmt.Errorf("dynamic tool %q definition must be an object", toolName)
	}

	row, err := dynamicRESTToolFromMap(defMap)
	if err != nil {
		return tools.ToolDefinition{}, false, err
	}
	if strings.TrimSpace(row.Name) == "" {
		row.Name = toolName
	}
	def, err := BuildDynamicRESTToolDefinition(row)
	if err != nil {
		return tools.ToolDefinition{}, false, err
	}
	return def, true, nil
}

func dynamicRESTToolFromMap(input map[string]interface{}) (database.DynamicRESTTool, error) {
	row := database.DynamicRESTTool{}
	row.Name, _ = input["name"].(string)
	row.FunctionName, _ = input["function_name"].(string)
	row.Description, _ = input["description"].(string)
	row.AdminOnly, _ = input["admin_only"].(bool)
	row.RequiresConfirmation, _ = input["requires_confirmation"].(bool)
	row.StopOnFirstConfirmableToolCall, _ = input["stop_on_first_confirmable_tool_call"].(bool)
	row.ConfirmationBlockMessage, _ = input["confirmation_block_message"].(string)
	row.OpenAPISourceType, _ = input["openapi_source_type"].(string)
	row.OpenAPISource, _ = input["openapi_source"].(string)
	row.OperationID, _ = input["operation_id"].(string)
	row.HTTPMethod, _ = input["http_method"].(string)
	row.Path, _ = input["path"].(string)
	row.BaseURLSource, _ = input["base_url_source"].(string)
	row.BaseURLInputName, _ = input["base_url_input_name"].(string)

	paramBindingsJSON, err := json.Marshal(input["param_bindings"])
	if err != nil {
		return row, fmt.Errorf("invalid param_bindings: %w", err)
	}
	row.ParamBindings = paramBindingsJSON

	safetyPolicyJSON, err := json.Marshal(input["safety_policy"])
	if err != nil {
		return row, fmt.Errorf("invalid safety_policy: %w", err)
	}
	row.SafetyPolicy = safetyPolicyJSON
	return row, nil
}

func parseJSONArrayOrEmpty(raw json.RawMessage) []map[string]interface{} {
	if len(raw) == 0 {
		return []map[string]interface{}{}
	}
	out := []map[string]interface{}{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func parseJSONObjectOrEmpty(raw json.RawMessage) map[string]interface{} {
	if len(raw) == 0 {
		return map[string]interface{}{}
	}
	out := map[string]interface{}{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func BuildDynamicRESTToolDefinition(row database.DynamicRESTTool) (tools.ToolDefinition, error) {
	if strings.TrimSpace(row.Name) == "" {
		return tools.ToolDefinition{}, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(row.OpenAPISourceType) == "" {
		row.OpenAPISourceType = "inline"
	}

	doc, err := loadOpenAPIDocument(row.OpenAPISourceType, row.OpenAPISource)
	if err != nil {
		return tools.ToolDefinition{}, err
	}
	baseURL, params, method, path, err := resolveOperation(doc, row.OperationID, row.HTTPMethod, row.Path)
	if err != nil {
		return tools.ToolDefinition{}, err
	}

	bindings, safety, err := compileBindingsAndPolicy(params, row.ParamBindings, row.SafetyPolicy)
	if err != nil {
		return tools.ToolDefinition{}, err
	}

	callSchema := map[string]interface{}{"type": "object", "properties": map[string]interface{}{}, "required": []string{}, "additionalProperties": false}
	initSchema := map[string]interface{}{"type": "object", "properties": map[string]interface{}{}, "required": []string{}, "additionalProperties": false}
	callProps := callSchema["properties"].(map[string]interface{})
	initProps := initSchema["properties"].(map[string]interface{})
	callRequired := []string{}
	initRequired := []string{}

	baseURLSource := strings.ToLower(strings.TrimSpace(row.BaseURLSource))
	baseURLInputName := strings.TrimSpace(row.BaseURLInputName)
	if baseURLSource == "" && baseURLInputName != "" {
		baseURLSource = "init"
	}
	if baseURLSource != "" && baseURLSource != "init" && baseURLSource != "call" {
		return tools.ToolDefinition{}, fmt.Errorf("base_url_source must be 'init' or 'call'")
	}
	if baseURLSource != "" && baseURLInputName == "" {
		return tools.ToolDefinition{}, fmt.Errorf("base_url_input_name is required when base_url_source is set")
	}
	if baseURLInputName != "" {
		baseURLSchema := map[string]interface{}{
			"type":        "string",
			"description": "Base URL override for this REST tool",
		}
		baseURLRequired := strings.TrimSpace(baseURL) == ""
		if baseURLSource == "init" {
			initProps[baseURLInputName] = baseURLSchema
			if baseURLRequired {
				initRequired = append(initRequired, baseURLInputName)
			}
		} else {
			callProps[baseURLInputName] = baseURLSchema
			if baseURLRequired {
				callRequired = append(callRequired, baseURLInputName)
			}
		}
	}

	for _, binding := range bindings {
		prop := map[string]interface{}{"type": "string"}
		if schema := bindingSchema(params, binding.In, binding.Name); schema != nil {
			prop = cloneMap(schema)
		}
		if binding.Description != "" {
			prop["description"] = binding.Description
		}
		if binding.Source == "init" {
			initProps[binding.InputName] = prop
			if binding.Required {
				initRequired = append(initRequired, binding.InputName)
			}
		} else {
			callProps[binding.InputName] = prop
			if binding.Required {
				callRequired = append(callRequired, binding.InputName)
			}
		}
	}
	callSchema["required"] = callRequired
	initSchema["required"] = initRequired

	spec := dynamicRESTToolSpec{Method: method, Path: path, BaseURL: baseURL, BaseURLSource: baseURLSource, BaseURLInputName: baseURLInputName, Bindings: bindings, Safety: safety}

	functionName := strings.TrimSpace(row.FunctionName)
	if functionName == "" {
		functionName = row.Name
	}

	def := tools.ToolDefinition{
		Name:                           row.Name,
		FunctionName:                   functionName,
		Description:                    row.Description,
		AdminOnly:                      row.AdminOnly,
		RequiresInit:                   len(initProps) > 0,
		InitSchema:                     initSchema,
		RequiresConfirmation:           row.RequiresConfirmation,
		StopOnFirstConfirmableToolCall: row.StopOnFirstConfirmableToolCall,
		ConfirmationBlockMessage:       row.ConfirmationBlockMessage,
		InputType:                      map[string]interface{}{},
		InputSchema:                    callSchema,
		RunFunction:                    runDynamicRESTTool(spec),
	}
	return def, nil
}

func runDynamicRESTTool(spec dynamicRESTToolSpec) func(input interface{}, init map[string]interface{}) (string, error) {
	return func(input interface{}, init map[string]interface{}) (string, error) {
		callInput := normalizeInterfaceMap(input)
		initInput := init
		if initInput == nil {
			initInput = map[string]interface{}{}
		}

		baseURL := strings.TrimSpace(spec.BaseURL)
		if spec.BaseURLInputName != "" {
			var source map[string]interface{}
			if spec.BaseURLSource == "call" {
				source = callInput
			} else {
				source = initInput
			}
			if raw, exists := source[spec.BaseURLInputName]; exists {
				if resolved := strings.TrimSpace(fmt.Sprintf("%v", raw)); resolved != "" {
					baseURL = resolved
				}
			}
		}

		if err := validateOutboundURL(baseURL, spec.Safety); err != nil {
			return "", err
		}

		path := spec.Path
		query := url.Values{}
		headers := http.Header{}
		cookies := []*http.Cookie{}
		body := map[string]interface{}{}

		for _, binding := range spec.Bindings {
			var source map[string]interface{}
			if binding.Source == "init" {
				source = initInput
			} else {
				source = callInput
			}
			value, exists := source[binding.InputName]
			if !exists {
				if binding.Required {
					return "", fmt.Errorf("missing required %s field %q", binding.Source, binding.InputName)
				}
				continue
			}
			valueStr := fmt.Sprintf("%v", value)
			switch binding.In {
			case "path":
				path = strings.ReplaceAll(path, "{"+binding.Name+"}", url.PathEscape(valueStr))
			case "query":
				query.Set(binding.Name, valueStr)
			case "header":
				headers.Set(binding.Name, valueStr)
			case "cookie":
				cookies = append(cookies, &http.Cookie{Name: binding.Name, Value: valueStr})
			case "body":
				body[binding.Name] = value
			}
		}

		requestURL := strings.TrimRight(baseURL, "/") + path
		if encoded := query.Encode(); encoded != "" {
			requestURL += "?" + encoded
		}

		var bodyReader io.Reader
		if len(body) > 0 {
			encoded, err := json.Marshal(body)
			if err != nil {
				return "", err
			}
			bodyReader = bytes.NewReader(encoded)
			headers.Set("Content-Type", "application/json")
		}

		req, err := http.NewRequest(spec.Method, requestURL, bodyReader)
		if err != nil {
			return "", err
		}
		for key, values := range headers {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
		for _, c := range cookies {
			req.AddCookie(c)
		}

		timeout := spec.Safety.TimeoutSeconds
		if timeout <= 0 {
			timeout = 20
		}
		client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		maxBytes := spec.Safety.MaxResponseBody
		if maxBytes <= 0 {
			maxBytes = 64 * 1024
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
		if err != nil {
			return "", err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("received non-success status %d: %s", resp.StatusCode, string(data))
		}

		filteredResult, err := applyResponseCensorPaths(data, spec.Safety.ResponseCensorPaths)
		if err != nil {
			return "", err
		}
		return filteredResult, nil
	}
}

func compileBindingsAndPolicy(params []operationParam, bindingsRaw json.RawMessage, safetyRaw json.RawMessage) ([]compiledBinding, restToolSafetyPolicy, error) {
	available := map[string]operationParam{}
	for _, p := range params {
		available[p.In+":"+p.Name] = p
	}

	bindings := []restToolParamBinding{}
	if len(bindingsRaw) > 0 {
		if err := json.Unmarshal(bindingsRaw, &bindings); err != nil {
			return nil, restToolSafetyPolicy{}, fmt.Errorf("invalid param_bindings: %w", err)
		}
	}
	if len(bindings) == 0 {
		for _, p := range params {
			bindings = append(bindings, restToolParamBinding{InputName: p.Name, Source: "call", In: p.In, Name: p.Name})
		}
	}

	out := make([]compiledBinding, 0, len(bindings))
	for idx, binding := range bindings {
		binding.InputName = strings.TrimSpace(binding.InputName)
		binding.Source = strings.ToLower(strings.TrimSpace(binding.Source))
		binding.In = strings.ToLower(strings.TrimSpace(binding.In))
		binding.Name = strings.TrimSpace(binding.Name)
		if binding.InputName == "" || binding.Name == "" || binding.In == "" {
			return nil, restToolSafetyPolicy{}, fmt.Errorf("param_bindings[%d] requires input_name, in, and name", idx)
		}
		if binding.Source != "call" && binding.Source != "init" {
			return nil, restToolSafetyPolicy{}, fmt.Errorf("param_bindings[%d] source must be 'call' or 'init'", idx)
		}
		param, ok := available[binding.In+":"+binding.Name]
		if !ok {
			return nil, restToolSafetyPolicy{}, fmt.Errorf("param_bindings[%d] references unknown operation parameter %s:%s", idx, binding.In, binding.Name)
		}
		required := param.Required
		if binding.Required != nil {
			required = *binding.Required
		}
		out = append(out, compiledBinding{restToolParamBinding: binding, Required: required})
	}

	policy := restToolSafetyPolicy{}
	if len(safetyRaw) > 0 {
		if err := json.Unmarshal(safetyRaw, &policy); err != nil {
			return nil, restToolSafetyPolicy{}, fmt.Errorf("invalid safety_policy: %w", err)
		}
	}
	for i, host := range policy.AllowHosts {
		policy.AllowHosts[i] = strings.ToLower(strings.TrimSpace(host))
	}

	validatedCensorPaths := make([]string, 0, len(policy.ResponseCensorPaths))
	for idx, rawPath := range policy.ResponseCensorPaths {
		path := strings.TrimSpace(rawPath)
		if path == "" {
			return nil, restToolSafetyPolicy{}, fmt.Errorf("safety_policy.response_censor_paths[%d] must not be empty", idx)
		}
		if err := validateResponseCensorPath(path); err != nil {
			return nil, restToolSafetyPolicy{}, fmt.Errorf("safety_policy.response_censor_paths[%d] %w", idx, err)
		}
		validatedCensorPaths = append(validatedCensorPaths, path)
	}
	policy.ResponseCensorPaths = validatedCensorPaths

	return out, policy, nil
}

func validateResponseCensorPath(path string) error {
	segments := strings.Split(path, ".")
	if len(segments) == 0 {
		return fmt.Errorf("is invalid")
	}

	for idx, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			return fmt.Errorf("contains an empty path segment")
		}
		if strings.Contains(segment, "*") && segment != "*" {
			return fmt.Errorf("contains unsupported wildcard segment %q", segment)
		}
		if segment == "*" && idx == len(segments)-1 {
			return fmt.Errorf("cannot end with wildcard '*' segment")
		}
	}

	return nil
}

func applyResponseCensorPaths(data []byte, censorPaths []string) (string, error) {
	if len(censorPaths) == 0 {
		return string(data), nil
	}

	var payload interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("response_censor_paths requires JSON response body: %w", err)
	}

	for _, path := range censorPaths {
		segments := strings.Split(path, ".")
		removeCensoredPath(payload, segments)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to encode censored response body: %w", err)
	}

	return string(encoded), nil
}

func removeCensoredPath(node interface{}, segments []string) {
	if len(segments) == 0 {
		return
	}

	segment := segments[0]
	lastSegment := len(segments) == 1

	switch typed := node.(type) {
	case map[string]interface{}:
		if segment == "*" {
			if lastSegment {
				return
			}
			for _, child := range typed {
				removeCensoredPath(child, segments[1:])
			}
			return
		}

		if lastSegment {
			delete(typed, segment)
			return
		}

		child, exists := typed[segment]
		if !exists {
			return
		}
		removeCensoredPath(child, segments[1:])

	case []interface{}:
		if segment == "*" {
			if lastSegment {
				return
			}
			for _, child := range typed {
				removeCensoredPath(child, segments[1:])
			}
			return
		}

		index, err := strconv.Atoi(segment)
		if err != nil {
			return
		}
		if index < 0 || index >= len(typed) {
			return
		}
		if lastSegment {
			typed[index] = nil
			return
		}
		removeCensoredPath(typed[index], segments[1:])
	}
}

func loadOpenAPIDocument(sourceType string, source string) (map[string]interface{}, error) {
	var data []byte
	sourceType = strings.ToLower(strings.TrimSpace(sourceType))
	if sourceType == "url" {
		parsed, err := url.Parse(strings.TrimSpace(source))
		if err != nil {
			return nil, fmt.Errorf("invalid openapi source url: %w", err)
		}
		if parsed.Scheme != "https" && parsed.Scheme != "http" {
			return nil, fmt.Errorf("openapi source url must use http or https")
		}
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Get(parsed.String())
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("failed to fetch openapi source: status %d", resp.StatusCode)
		}
		data, err = io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		if err != nil {
			return nil, err
		}
	} else {
		data = []byte(source)
	}

	doc := map[string]interface{}{}
	if err := json.Unmarshal(data, &doc); err == nil {
		return doc, nil
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid openapi document: %w", err)
	}
	return doc, nil
}

func resolveOperation(doc map[string]interface{}, operationID, methodRaw, pathRaw string) (string, []operationParam, string, string, error) {
	paths, ok := doc["paths"].(map[string]interface{})
	if !ok {
		return "", nil, "", "", fmt.Errorf("openapi document has no paths object")
	}

	findByOperationID := strings.TrimSpace(operationID) != ""
	method := strings.ToLower(strings.TrimSpace(methodRaw))
	path := strings.TrimSpace(pathRaw)

	selectedPath := ""
	selectedMethod := ""
	var selectedOp map[string]interface{}
	var pathItem map[string]interface{}

	for candidatePath, pathValue := range paths {
		pathMap, ok := pathValue.(map[string]interface{})
		if !ok {
			continue
		}
		for methodKey, opValue := range pathMap {
			opMap, ok := opValue.(map[string]interface{})
			if !ok {
				continue
			}
			methodKeyLower := strings.ToLower(strings.TrimSpace(methodKey))
			if findByOperationID {
				if opID, _ := opMap["operationId"].(string); opID == operationID {
					selectedPath = candidatePath
					selectedMethod = methodKeyLower
					selectedOp = opMap
					pathItem = pathMap
					break
				}
			} else if candidatePath == path && methodKeyLower == method {
				selectedPath = candidatePath
				selectedMethod = methodKeyLower
				selectedOp = opMap
				pathItem = pathMap
				break
			}
		}
		if selectedOp != nil {
			break
		}
	}
	if selectedOp == nil {
		if findByOperationID {
			return "", nil, "", "", fmt.Errorf("operationId %q not found", operationID)
		}
		return "", nil, "", "", fmt.Errorf("operation %s %s not found", method, path)
	}

	params := collectOperationParams(doc, pathItem, selectedOp)
	baseURL := resolveBaseURL(doc)
	return baseURL, params, strings.ToUpper(selectedMethod), selectedPath, nil
}

func collectOperationParams(doc map[string]interface{}, pathItem map[string]interface{}, operation map[string]interface{}) []operationParam {
	result := []operationParam{}
	seen := map[string]struct{}{}
	appendParams := func(raw interface{}) {
		rows, ok := raw.([]interface{})
		if !ok {
			return
		}
		for _, entry := range rows {
			paramMap, ok := normalizeRefObject(doc, entry)
			if !ok {
				continue
			}
			inValue, _ := paramMap["in"].(string)
			nameValue, _ := paramMap["name"].(string)
			if inValue == "" || nameValue == "" {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(inValue)) + ":" + strings.TrimSpace(nameValue)
			if _, exists := seen[key]; exists {
				continue
			}
			required, _ := paramMap["required"].(bool)
			schema := resolveSchemaObject(doc, paramMap["schema"])
			description, _ := paramMap["description"].(string)
			result = append(result, operationParam{In: strings.ToLower(strings.TrimSpace(inValue)), Name: strings.TrimSpace(nameValue), Required: required, Description: description, Schema: schema})
			seen[key] = struct{}{}
		}
	}

	appendParams(pathItem["parameters"])
	appendParams(operation["parameters"])

	requestBody, ok := normalizeRefObject(doc, operation["requestBody"])
	if ok {
		required, _ := requestBody["required"].(bool)
		if content, ok := requestBody["content"].(map[string]interface{}); ok {
			if jsonContent, ok := content["application/json"].(map[string]interface{}); ok {
				schema := resolveSchemaObject(doc, jsonContent["schema"])
				if schema != nil {
					if props, ok := schema["properties"].(map[string]interface{}); ok {
						for propName, rawPropSchema := range props {
							propSchema := resolveSchemaObject(doc, rawPropSchema)
							isRequired := required
							if reqList, ok := schema["required"].([]interface{}); ok {
								isRequired = false
								for _, reqEntry := range reqList {
									if reqName, ok := reqEntry.(string); ok && reqName == propName {
										isRequired = true
										break
									}
								}
							}
							result = append(result, operationParam{In: "body", Name: propName, Required: isRequired, Schema: propSchema})
						}
					}
				}
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].In == result[j].In {
			return result[i].Name < result[j].Name
		}
		return result[i].In < result[j].In
	})
	return result
}

func resolveBaseURL(doc map[string]interface{}) string {
	servers, ok := doc["servers"].([]interface{})
	if !ok || len(servers) == 0 {
		return ""
	}
	first, ok := servers[0].(map[string]interface{})
	if !ok {
		return ""
	}
	urlValue, _ := first["url"].(string)
	return strings.TrimSpace(urlValue)
}

func normalizeRefObject(doc map[string]interface{}, raw interface{}) (map[string]interface{}, bool) {
	obj, ok := raw.(map[string]interface{})
	if !ok {
		return nil, false
	}
	if ref, hasRef := obj["$ref"].(string); hasRef && ref != "" {
		resolved := resolveRef(doc, ref)
		if resolved == nil {
			return nil, false
		}
		return resolved, true
	}
	return obj, true
}

func resolveSchemaObject(doc map[string]interface{}, raw interface{}) map[string]interface{} {
	obj, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	if ref, hasRef := obj["$ref"].(string); hasRef && ref != "" {
		resolved := resolveRef(doc, ref)
		if resolved == nil {
			return nil
		}
		return cloneMap(resolved)
	}
	return cloneMap(obj)
}

func resolveRef(doc map[string]interface{}, ref string) map[string]interface{} {
	if !strings.HasPrefix(ref, "#/") {
		return nil
	}
	segments := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
	var current interface{} = doc
	for _, segment := range segments {
		nextMap, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = nextMap[segment]
	}
	resolved, _ := current.(map[string]interface{})
	return resolved
}

func bindingSchema(params []operationParam, inValue, name string) map[string]interface{} {
	for _, p := range params {
		if p.In == inValue && p.Name == name {
			if p.Schema != nil {
				return p.Schema
			}
			break
		}
	}
	return nil
}

func cloneMap(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}
	encoded, _ := json.Marshal(input)
	cloned := map[string]interface{}{}
	_ = json.Unmarshal(encoded, &cloned)
	return cloned
}

func normalizeInterfaceMap(input interface{}) map[string]interface{} {
	if input == nil {
		return map[string]interface{}{}
	}
	if asMap, ok := input.(map[string]interface{}); ok {
		return asMap
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return map[string]interface{}{}
	}
	result := map[string]interface{}{}
	_ = json.Unmarshal(encoded, &result)
	return result
}

func validateOutboundURL(baseURL string, policy restToolSafetyPolicy) error {
	if strings.TrimSpace(baseURL) == "" {
		return fmt.Errorf("openapi operation has no server url")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("unsupported URL scheme")
	}

	hostname := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if hostname == "" {
		return fmt.Errorf("server url host is required")
	}
	if len(policy.AllowHosts) > 0 {
		allowed := false
		for _, host := range policy.AllowHosts {
			if host == hostname {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("host %q is not in allow_hosts", hostname)
		}
	}

	if policy.AllowPrivateIPs {
		return nil
	}
	if ip := net.ParseIP(hostname); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("host resolves to private address")
		}
		return nil
	}
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("host resolves to private address")
		}
	}
	return nil
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast()
}
