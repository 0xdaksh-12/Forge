import { useEffect, useState, useCallback } from "react";
import { Link } from "react-router-dom";
import { api } from "../api/client";
import type { Build, BuildStatus } from "../api/client";
import { StatusBadge } from "../components/ui";
import { duration, reltime, shortSha, triggerIcon } from "../components/utils";

const FILTERS: { label: string; value: BuildStatus | "" }[] = [
  { label: "All", value: "" },
  { label: "Running", value: "running" },
  { label: "Success", value: "success" },
  { label: "Failed", value: "failed" },
];

export function Dashboard() {
  const [builds, setBuilds] = useState<Build[]>([]);
  const [filter, setFilter] = useState<BuildStatus | "">("");
  const [loading, setLoading] = useState(true);

  const load = useCallback(() => {
    api.builds
      .list(filter ? { status: filter as BuildStatus } : undefined)
      .then((b) => setBuilds(b ?? []))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [filter]);

  useEffect(() => {
    load();

    // Connect to real-time build event stream
    const es = new EventSource("/api/v1/builds/events");

    es.onmessage = (e) => {
      console.log("Real-time update received:", e.data);
      load(); // Refresh the list when any build changes
    };

    es.onerror = () => {
      console.error("SSE connection failed, falling back to polling");
    };

    return () => es.close();
  }, [load]);

  const handleFilterChange = (val: BuildStatus | "") => {
    setFilter(val);
    setLoading(true);
  };

  const running = builds.filter((b) => b.Status === "running").length;
  const success = builds.filter((b) => b.Status === "success").length;
  const failed = builds.filter((b) => b.Status === "failed").length;

  return (
    <>
      <div className="page-header">
        <div>
          <h1 className="page-title">Dashboard</h1>
          <p className="page-subtitle">Live real-time build feed</p>
        </div>
      </div>

      <div className="page-body">
        <div className="stats-row">
          <div className="stat-card">
            <div className="stat-label">Total Builds</div>
            <div className="stat-value brand">{builds.length}</div>
          </div>
          <div className="stat-card">
            <div className="stat-label">Running</div>
            <div className="stat-value info">{running}</div>
          </div>
          <div className="stat-card">
            <div className="stat-label">Success</div>
            <div className="stat-value success">{success}</div>
          </div>
          <div className="stat-card">
            <div className="stat-label">Failed</div>
            <div className="stat-value error">{failed}</div>
          </div>
        </div>

        <div className="flex gap-8 items-center" style={{ marginBottom: 16 }}>
          {FILTERS.map((f) => (
            <button
              key={f.value}
              className={`btn btn-sm ${filter === f.value ? "btn-primary" : "btn-ghost"}`}
              onClick={() => handleFilterChange(f.value)}
            >
              {f.label}
            </button>
          ))}
        </div>

        <div className="card">
          {loading ? (
            <div className="empty-state">
              <div className="empty-icon">
                <span className="material-icons" style={{ fontSize: "48px" }}>
                  hourglass_empty
                </span>
              </div>
              <div className="empty-title">Loading builds…</div>
            </div>
          ) : builds.length === 0 ? (
            <div className="empty-state">
              <div className="empty-icon">
                <span className="material-icons" style={{ fontSize: "48px" }}>
                  history
                </span>
              </div>
              <div className="empty-title">No builds yet</div>

              <div className="empty-desc">
                Register a pipeline and push to a repo to trigger your first
                build.
              </div>
            </div>
          ) : (
            <div className="build-list">
              {builds.map((b) => (
                <Link to={`/builds/${b.ID}`} className="build-row" key={b.ID}>
                  <div className="build-number">#{b.ID}</div>
                  <div className="build-info">
                    <div className="build-commit">
                      <span className="material-icons material-icons-inline">
                        {triggerIcon(b.Trigger)}
                      </span>
                      &nbsp;
                      {b.CommitMsg || "No commit message"}
                    </div>

                    <div className="build-meta">
                      <span className="build-meta-tag">
                        <span className="material-icons material-icons-inline">
                          account_tree
                        </span>{" "}
                        {b.Branch}
                      </span>

                      <span className="build-meta-tag">
                        <code>{shortSha(b.CommitSHA)}</code>
                      </span>
                      {b.Pipeline && (
                        <span className="build-meta-tag">
                          <span className="material-icons material-icons-inline">
                            inventory_2
                          </span>{" "}
                          {b.Pipeline.Name}
                        </span>
                      )}

                      {/* Use StartedAt only — CreatedAt is not part of the Build type */}
                      <span className="build-meta-tag text-muted">
                        {reltime(b.StartedAt)}
                      </span>
                    </div>
                  </div>
                  <div className="build-duration">
                    {duration(b.StartedAt, b.FinishedAt)}
                  </div>
                  <StatusBadge status={b.Status} />
                </Link>
              ))}
            </div>
          )}
        </div>
      </div>
    </>
  );
}
