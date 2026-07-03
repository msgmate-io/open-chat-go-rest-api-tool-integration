# REST API Tool Integration

The REST API Tool integration allows users to define dynamic tools from OpenAPI operations without recompiling the backend.

## What it provides

- Dynamic REST tool CRUD APIs under `/api/v1/integrations/rest_api_tool/tools`
- Per-tool detail API by UUID
- OpenAPI operation parsing (method/path/params/request body)
- Safe outbound execution controls (allow-hosts, private-IP policy, timeout, response size, response censor paths)
- Frontend pages:
  - `/integrations/rest_api_tool/tools`
  - `/integrations/rest_api_tool/tools/{tool_uuid}`

## Typical usage

1. Create a dynamic REST tool definition from OpenAPI spec + target operation
2. Optionally configure init/call bindings and safety policy
3. Add tool name to bot `default_shared_config.tools`
4. Provide `tool_init` if required by generated init schema

When bot config is saved, matching user-owned dynamic REST tools are snapshotted into `dynamic_tools` and executed at runtime.
