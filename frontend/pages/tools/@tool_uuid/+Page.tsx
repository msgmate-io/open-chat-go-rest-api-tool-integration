import { useMemo } from "react";
import useSWR from "swr";
import { usePageContext } from "vike-react/usePageContext";
import { IntegrationPageShell } from "@open-chat-go/ui";
import { fetcher } from "@/lib/utils";
import {
  Badge,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Text,
  TextTypes,
} from "@open-chat-go/ui";

type DynamicRESTToolRow = {
  uuid: string;
  name: string;
  function_name: string;
  description: string;
  enabled: boolean;
  openapi_source_type: string;
  operation_id: string;
  http_method: string;
  path: string;
  base_url_source: string;
  base_url_input_name: string;
  param_bindings: Array<Record<string, unknown>>;
  safety_policy: Record<string, unknown>;
};

type DynamicRESTToolDetailResponse = {
  row: DynamicRESTToolRow;
  call_schema: Record<string, unknown>;
  init_schema: Record<string, unknown>;
};

function resolveToolUUID(pathname: string, fallback: string): string {
  const parts = pathname.split("/").filter(Boolean);
  const fromPath = parts[parts.length - 1] || "";
  return String(fromPath || fallback || "").trim();
}

export default function RESTAPIToolDetailPage() {
  const pageContext = usePageContext();
  const fallbackUUID = String(pageContext.routeParams.tool_uuid || "").trim();
  const toolUUID = useMemo(() => {
    if (typeof window === "undefined") {
      return fallbackUUID;
    }
    return resolveToolUUID(window.location.pathname, fallbackUUID);
  }, [fallbackUUID]);

  const encodedUUID = encodeURIComponent(toolUUID);

  const { data, isLoading, error } = useSWR<DynamicRESTToolDetailResponse>(
    encodedUUID ? `/api/v1/integrations/rest_api_tool/tools/${encodedUUID}` : null,
    fetcher,
  );

  const row = data?.row;

  return (
    <IntegrationPageShell>
      <div className="space-y-4">
        <div>
          <Text type={TextTypes.Heading5} tag="h1" bold>
            REST API Tool Details
          </Text>
          <Text type={TextTypes.Body6} color="muted" className="font-mono break-all">
            {toolUUID || "Unknown tool UUID"}
          </Text>
        </div>

        {isLoading ? <div className="h-28 animate-pulse rounded bg-muted" /> : null}
        {error ? (
          <Card className="border-destructive/30 bg-destructive/5">
            <CardHeader>
              <CardTitle className="text-base">Failed to load tool details</CardTitle>
              <CardDescription>Please verify this tool exists and you have access.</CardDescription>
            </CardHeader>
          </Card>
        ) : null}

        {!isLoading && !error && row ? (
          <>
            <Card>
              <CardHeader>
                <CardTitle>{row.name}</CardTitle>
                <CardDescription>{row.description || "No description"}</CardDescription>
              </CardHeader>
              <CardContent className="space-y-2">
                <div className="flex flex-wrap gap-2">
                  <Badge variant={row.enabled ? "secondary" : "outline"}>
                    {row.enabled ? "enabled" : "disabled"}
                  </Badge>
                  {row.http_method ? <Badge variant="outline">{row.http_method}</Badge> : null}
                  {row.path ? <Badge variant="outline" className="font-mono break-all">{row.path}</Badge> : null}
                </div>
                <div className="grid grid-cols-1 gap-2 text-sm text-muted-foreground md:grid-cols-2">
                  <div>
                    <span className="font-medium">UUID:</span> <span className="font-mono break-all">{row.uuid}</span>
                  </div>
                  <div>
                    <span className="font-medium">Function:</span>{" "}
                    <span className="font-mono break-all">{row.function_name || "-"}</span>
                  </div>
                  <div>
                    <span className="font-medium">OpenAPI Source:</span> {row.openapi_source_type || "-"}
                  </div>
                  <div>
                    <span className="font-medium">Operation ID:</span>{" "}
                    <span className="font-mono break-all">{row.operation_id || "-"}</span>
                  </div>
                  <div>
                    <span className="font-medium">Base URL Source:</span> {row.base_url_source || "-"}
                  </div>
                  <div>
                    <span className="font-medium">Base URL Input:</span>{" "}
                    <span className="font-mono break-all">{row.base_url_input_name || "-"}</span>
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">Call Schema</CardTitle>
              </CardHeader>
              <CardContent>
                <pre className="max-h-80 overflow-auto rounded bg-muted/30 p-3 text-xs">
                  {JSON.stringify(data.call_schema ?? {}, null, 2)}
                </pre>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">Init Schema</CardTitle>
              </CardHeader>
              <CardContent>
                <pre className="max-h-80 overflow-auto rounded bg-muted/30 p-3 text-xs">
                  {JSON.stringify(data.init_schema ?? {}, null, 2)}
                </pre>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">Bindings & Safety</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div>
                  <Text type={TextTypes.Body6} bold>
                    Param Bindings
                  </Text>
                  <pre className="mt-1 max-h-64 overflow-auto rounded bg-muted/30 p-3 text-xs">
                    {JSON.stringify(row.param_bindings ?? [], null, 2)}
                  </pre>
                </div>
                <div>
                  <Text type={TextTypes.Body6} bold>
                    Safety Policy
                  </Text>
                  <pre className="mt-1 max-h-64 overflow-auto rounded bg-muted/30 p-3 text-xs">
                    {JSON.stringify(row.safety_policy ?? {}, null, 2)}
                  </pre>
                </div>
              </CardContent>
            </Card>
          </>
        ) : null}
      </div>
    </IntegrationPageShell>
  );
}
