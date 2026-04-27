// Single-export component file — satisfies fast-refresh requirement.
import type { BuildStatus, JobStatus } from "../api/client";
import { STATUS_LABELS } from "./utils";

interface BadgeProps {
  status: BuildStatus | JobStatus;
}

export function StatusBadge({ status }: BadgeProps) {
  return (
    <span className={`badge ${status}`}>
      <span className="badge-dot" />
      {STATUS_LABELS[status] ?? status}
    </span>
  );
}
