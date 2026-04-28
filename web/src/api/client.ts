// Typed API client — all calls go through this module.
// Base URL is relative so it works with the Vite proxy in dev
// and the embedded static server in production.

const BASE = import.meta.env.VITE_API_BASE ?? "";
export const TOKEN = import.meta.env.VITE_API_TOKEN ?? "forge-secret";

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      "X-Forge-Token": TOKEN,
      ...options.headers,
    },
  });
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText);
    throw new Error(`${res.status}: ${text}`);
  }
  return res.json() as Promise<T>;
}

export type BuildStatus =
  | "pending"
  | "running"
  | "success"
  | "failed"
  | "cancelled";
export type JobStatus =
  | "pending"
  | "running"
  | "success"
  | "failed"
  | "skipped";
export type TriggerType = "push" | "pull_request" | "manual";

export interface Pipeline {
  ID: number;
  Name: string;
  RepoURL: string;
  DefaultBranch: string;
  GitHubRepo: string;
  ConfigPath: string;
  CreatedAt: string;
}

export interface Build {
  ID: number;
  PipelineID: number;
  Pipeline?: Pipeline;
  Trigger: TriggerType;
  CommitSHA: string;
  Branch: string;
  CommitMsg: string;
  AuthorName: string;
  Status: BuildStatus;
  StartedAt?: string;
  FinishedAt?: string;
  Jobs?: Job[];
}

export interface Job {
  ID: number;
  BuildID: number;
  Name: string;
  Image: string;
  Status: JobStatus;
  ExitCode: number;
  StartedAt?: string;
  FinishedAt?: string;
}

export interface LogLine {
  ID: number;
  JobID: number;
  Seq: number;
  Stream: "stdout" | "stderr";
  Text: string;
  Timestamp: string;
}

export interface Secret {
  ID: number;
  PipelineID: number;
  Name: string;
  CreatedAt: string;
  UpdatedAt: string;
}

export const api = {
  pipelines: {
    list: () => request<Pipeline[]>("/api/v1/pipelines"),
    get: (id: number) =>
      request<{ pipeline: Pipeline; builds: Build[] }>(
        `/api/v1/pipelines/${id}`,
      ),
    create: (data: Partial<Pipeline>) =>
      request<Pipeline>("/api/v1/pipelines", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    delete: (id: number) =>
      fetch(`${BASE}/api/v1/pipelines/${id}`, {
        method: "DELETE",
        headers: { "X-Forge-Token": TOKEN },
      }),
    secrets: {
      list: (pipelineId: number) =>
        request<Secret[]>(`/api/v1/pipelines/${pipelineId}/secrets`),
      put: (pipelineId: number, name: string, value: string) =>
        request<{ status: string; name: string }>(
          `/api/v1/pipelines/${pipelineId}/secrets`,
          {
            method: "PUT",
            body: JSON.stringify({ name, value }),
          },
        ),
      delete: (pipelineId: number, name: string) =>
        fetch(`${BASE}/api/v1/pipelines/${pipelineId}/secrets/${name}`, {
          method: "DELETE",
          headers: { "X-Forge-Token": TOKEN },
        }),
    },
  },

  builds: {
    list: (params?: { pipeline_id?: number; status?: BuildStatus }) => {
      const qs = params
        ? "?" +
          new URLSearchParams(
            Object.entries(params).map(([k, v]) => [k, String(v)]),
          ).toString()
        : "";
      return request<Build[]>(`/api/v1/builds${qs}`);
    },
    get: (id: number) => request<Build>(`/api/v1/builds/${id}`),
    cancel: (id: number) =>
      request<{ status: string }>(`/api/v1/builds/${id}/cancel`, {
        method: "POST",
      }),
  },

  jobs: {
    get: (id: number) => request<Job>(`/api/v1/jobs/${id}`),
    logs: (id: number) => request<LogLine[]>(`/api/v1/jobs/${id}/logs`),
    streamUrl: (id: number) => `${BASE}/api/v1/jobs/${id}/logs/stream`,
  },

  webhooks: {
    trigger: (pipelineId: number, branch?: string) =>
      request<{ status: string }>(`/api/webhooks/manual/${pipelineId}`, {
        method: "POST",
        body: JSON.stringify({ branch }),
      }),
  },
  events: {
    buildsUrl: () => `${BASE}/api/v1/builds/events?token=${TOKEN}`,
    jobLogsUrl: (jobId: number) =>
      `${BASE}/api/v1/jobs/${jobId}/logs/stream?token=${TOKEN}`,
  },
};

