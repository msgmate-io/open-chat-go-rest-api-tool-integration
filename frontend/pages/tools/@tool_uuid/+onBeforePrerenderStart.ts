const PLACEHOLDER_TOOL_UUID = "tool-uuid-placeholder";

export default function onBeforePrerenderStart() {
  return [
    `/integrations/rest_api_tool/tools/${PLACEHOLDER_TOOL_UUID}`,
  ];
}
