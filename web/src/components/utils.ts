import type { BuildStatus, TriggerType } from "../api/client";

export function duration(start?: string, end?: string): string {
  if (!start) return "—";
  const ms = new Date(end ?? Date.now()).getTime() - new Date(start).getTime();
  if (ms < 0) return "—";
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  return `${m}m ${s % 60}s`;
}

const RTF = new Intl.RelativeTimeFormat("en", { numeric: "auto" });

export function reltime(iso?: string): string {
  if (!iso) return "—";
  const diff = (new Date(iso).getTime() - Date.now()) / 1000;
  if (Math.abs(diff) < 60) return RTF.format(Math.round(diff), "second");
  if (Math.abs(diff) < 3600) return RTF.format(Math.round(diff / 60), "minute");
  if (Math.abs(diff) < 86400)
    return RTF.format(Math.round(diff / 3600), "hour");
  return RTF.format(Math.round(diff / 86400), "day");
}

export function shortSha(sha: string): string {
  return sha ? sha.slice(0, 7) : "—";
}

export function triggerIcon(t: TriggerType | string): string {
  if (t === "push") return "publish";
  if (t === "pull_request") return "compare_arrows";
  return "play_arrow";
}

export const STATUS_LABELS: Record<BuildStatus | string, string> = {
  pending: "Pending",
  running: "Running",
  success: "Success",
  failed: "Failed",
  cancelled: "Cancelled",
  skipped: "Skipped",
};
