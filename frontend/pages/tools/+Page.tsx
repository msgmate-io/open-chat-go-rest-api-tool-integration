import useSWR from "swr";
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
  http_method: string;
  path: string;
  updated_at_unix: number;
};

type DynamicRESTToolsResponse = {
  rows: DynamicRESTToolRow[];
};

function formatUnixTimestamp(value: number): string {
  if (!value) {
    return "-";
  }
  try {
    return new Date(value * 1000).toLocaleString();
  } catch {
    return "-";
  }
}

export default function RESTAPIToolsListPage() {
  const { data, isLoading, error } = useSWR<DynamicRESTToolsResponse>(
    "/api/v1/integrations/rest_api_tool/tools",
    fetcher,
  );

  const rows = data?.rows ?? [];

  return (
    <IntegrationPageShell>
      <div className="space-y-4">
        <div>
          <Text type={TextTypes.Heading5} tag="h1" bold>
            Registered REST API Tools
          </Text>
          <Text type={TextTypes.Body6} color="muted">
            Dynamic tools configured for your account.
          </Text>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Tool List</CardTitle>
            <CardDescription>{rows.length} total</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {isLoading ? <div className="h-24 animate-pulse rounded bg-muted" /> : null}
            {error ? (
              <Text type={TextTypes.Body6} color="destructive">
                Failed to load REST API tools.
              </Text>
            ) : null}
            {!isLoading && !error && rows.length === 0 ? (
              <Text type={TextTypes.Body6} color="muted">
                No REST API tools configured yet.
              </Text>
            ) : null}

            {!isLoading && !error
              ? rows.map((row) => (
                  <a
                    key={row.uuid}
                    href={`/integrations/rest_api_tool/tools/${encodeURIComponent(row.uuid)}`}
                    className="block rounded-lg border border-border/70 bg-card/80 p-3 hover:bg-muted/20"
                  >
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <Text type={TextTypes.Body5} bold>
                          {row.name}
                        </Text>
                        <Badge variant={row.enabled ? "secondary" : "outline"}>
                          {row.enabled ? "enabled" : "disabled"}
                        </Badge>
                        {row.http_method ? <Badge variant="outline">{row.http_method}</Badge> : null}
                      </div>
                      <Text type={TextTypes.Body7} color="muted">
                        Updated: {formatUnixTimestamp(row.updated_at_unix)}
                      </Text>
                    </div>
                    <Text type={TextTypes.Body7} className="mt-2 font-mono">
                      {row.path || row.function_name || row.uuid}
                    </Text>
                    {row.description ? (
                      <Text type={TextTypes.Body7} color="muted" className="mt-1">
                        {row.description}
                      </Text>
                    ) : null}
                  </a>
                ))
              : null}
          </CardContent>
        </Card>
      </div>
    </IntegrationPageShell>
  );
}
