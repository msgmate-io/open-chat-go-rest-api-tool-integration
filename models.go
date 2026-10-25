package restapitoolintegration

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// This file owns the REST API tool integration's persistence models. They
// previously lived in backend/database and were shared with the core; the
// integration now owns them and registers DynamicRESTTool for migration through
// Definition.ModelProviders, so it no longer imports backend/... packages.
//
// The structs intentionally mirror the core schema field-for-field (table names,
// column tags and indexes) so existing databases and the core's own dynamic
// REST tool helpers keep working unchanged. The core keeps its own copy of
// DynamicRESTTool internally; the table is migrated once by the integration.

// Model is the shared gorm base model (UUID primary metadata plus timestamps),
// mirroring the core database.Model so integration tables keep their schema.
type Model struct {
	UUID      string         `gorm:"type:text;" json:"uuid"`
	ID        uint           `gorm:"primarykey" json:"-"`
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (b *Model) BeforeCreate(tx *gorm.DB) (err error) {
	b.UUID = uuid.New().String()
	return
}

// User is a read-only projection of the core users table used as the
// foreign-key target of DynamicRESTTool.OwnerUser. It is never registered for
// migration, so the core keeps ownership of the users schema.
type User struct {
	ID       uint   `gorm:"primarykey" json:"-"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"-"`
	IsAdmin  bool   `json:"is_admin"`
}

func (User) TableName() string {
	return "users"
}

// DynamicRESTTool stores runtime-configurable REST tool definitions that are
// loaded without recompiling the backend. The core keeps its own copy of this
// model for its internal helpers; this integration owns the migration.
type DynamicRESTTool struct {
	Model
	OwnerUserId                    uint            `json:"owner_user_id" gorm:"index;uniqueIndex:idx_dynamic_rest_tool_owner_name"`
	OwnerUser                      User            `json:"-" gorm:"foreignKey:OwnerUserId;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Name                           string          `json:"name" gorm:"size:160;uniqueIndex:idx_dynamic_rest_tool_owner_name"`
	FunctionName                   string          `json:"function_name" gorm:"size:160"`
	Description                    string          `json:"description" gorm:"type:text"`
	AdminOnly                      bool            `json:"admin_only" gorm:"default:false"`
	RequiresConfirmation           bool            `json:"requires_confirmation" gorm:"default:false"`
	StopOnFirstConfirmableToolCall bool            `json:"stop_on_first_confirmable_tool_call" gorm:"default:false"`
	ConfirmationBlockMessage       string          `json:"confirmation_block_message" gorm:"type:text"`
	Enabled                        bool            `json:"enabled" gorm:"default:true;index"`
	OpenAPISourceType              string          `json:"openapi_source_type" gorm:"size:16"`
	OpenAPISource                  string          `json:"openapi_source" gorm:"type:text"`
	OperationID                    string          `json:"operation_id" gorm:"size:255"`
	HTTPMethod                     string          `json:"http_method" gorm:"size:16"`
	Path                           string          `json:"path" gorm:"size:1024"`
	BaseURLSource                  string          `json:"base_url_source" gorm:"size:16"`
	BaseURLInputName               string          `json:"base_url_input_name" gorm:"size:128"`
	ParamBindings                  json.RawMessage `json:"param_bindings" gorm:"type:jsonb"`
	SafetyPolicy                   json.RawMessage `json:"safety_policy" gorm:"type:jsonb"`
}
