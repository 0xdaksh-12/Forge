import { useEffect, useState, useCallback } from "react";

import { useParams, Link } from "react-router-dom";
import { api, type Build, type Job } from "../api/client";
import { StatusBadge } from "../components/ui";
import { duration, reltime, shortSha, triggerIcon } from "../components/utils";
import { LogViewer } from "../components/LogViewer";

export function BuildDetail() {
  const { id } = useParams<{ id: string }>();
  const [build, setBuild] = useState<Build | null>(null);
  const [openJob, setOpenJob] = useState<number | null>(null);
  const [cancelling, setCancelling] = useState(false);

  const load = useCallback(() => {
    if (!id) return;
    api.builds
      .get(Number(id))
      .then((b) => {
        setBuild(b);
        // Auto-open the first running or failed job.
        if (b.Jobs && openJob === null) {
          const auto = b.Jobs.find(
            (j) => j.Status === "running" || j.Status === "failed",
          );
          if (auto) setOpenJob(auto.ID);
        }
      })
      .catch(() => {});
  }, [id, openJob]);

  useEffect(() => {
    load();
    const running = () =>
      build?.Status === "running" || build?.Status === "pending";
    const iv = setInterval(() => {
      if (running()) load();
    }, 3000);
    return () => clearInterval(iv);
  }, [load, build?.Status]);

  const handleCancel = async () => {
    if (!build) return;
    setCancelling(true);
    try {
      await api.builds.cancel(build.ID);
      load();
    } finally {
      setCancelling(false);
    }
  };

  if (!build)
    return (
      <div className="empty-state" style={{ paddingTop: 80 }}>
        <div className="empty-icon">
          <span className="material-icons" style={{ fontSize: "48px" }}>
            hourglass_empty
          </span>
        </div>
        <div className="empty-title">Loading build…</div>
      </div>
    );

  const isLive = build.Status === "running" || build.Status === "pending";

  return (
    <>
      <div className="page-header">
        <div>
          <div className="text-muted text-xs" style={{ marginBottom: 4 }}>
            <Link to="/">Dashboard</Link> / Build #{build.ID}
          </div>
          <h1 className="page-title">
            <span className="material-icons material-icons-inline">
              {triggerIcon(build.Trigger)}
            </span>
            &nbsp;
            {build.CommitMsg || "Build #" + build.ID}
          </h1>

          <p className="page-subtitle">
            <span className="material-icons material-icons-inline">
              account_tree
            </span>{" "}
            {build.Branch} · <code>{shortSha(build.CommitSHA)}</code>
            &nbsp;· {build.AuthorName} · {reltime(build.StartedAt)}
            &nbsp;·{" "}
            <span className="material-icons material-icons-inline">
              timer
            </span>{" "}
            {duration(build.StartedAt, build.FinishedAt)}
          </p>
        </div>
        <div className="flex gap-8 items-center">
          <StatusBadge status={build.Status} />
          {isLive && (
            <button
              className="btn btn-danger btn-sm"
              onClick={handleCancel}
              disabled={cancelling}
            >
              <span className="material-icons material-icons-inline">
                close
              </span>{" "}
              {cancelling ? "Cancelling…" : "Cancel"}
            </button>
          )}
        </div>
      </div>

      <div className="page-body">
        <div className="job-grid">
          {(build.Jobs ?? []).map((job) => (
            <JobCard
              key={job.ID}
              job={job}
              open={openJob === job.ID}
              onToggle={() =>
                setOpenJob((prev) => (prev === job.ID ? null : job.ID))
              }
            />
          ))}
          {(build.Jobs ?? []).length === 0 && (
            <div className="empty-state">
              <div className="empty-icon">
                <span className="material-icons" style={{ fontSize: "48px" }}>
                  hourglass_empty
                </span>
              </div>
              <div className="empty-title">Waiting for jobs to start…</div>
            </div>
          )}
        </div>
      </div>
    </>
  );
}

function JobCard({
  job,
  open,
  onToggle,
}: {
  job: Job;
  open: boolean;
  onToggle: () => void;
}) {
  const isLive = job.Status === "running";

  return (
    <div className={`job-card ${job.Status}`}>
      <div className="job-card-header" onClick={onToggle}>
        <div className="job-card-left">
          <StatusBadge status={job.Status} />
          <div>
            <div className="job-name">{job.Name}</div>
            {job.Image && <div className="job-image">{job.Image}</div>}
          </div>
        </div>
        <div className="job-card-right">
          <span className="job-duration">
            {duration(job.StartedAt, job.FinishedAt)}
          </span>
          {job.ExitCode !== -1 && job.ExitCode !== 0 && (
            <span style={{ fontSize: 11, color: "var(--error)" }}>
              exit {job.ExitCode}
            </span>
          )}
          <span className={`chevron${open ? " open" : ""}`}>
            <span className="material-icons" style={{ fontSize: "16px" }}>
              play_arrow
            </span>
          </span>
        </div>
      </div>
      {open && <LogViewer key={job.ID} jobId={job.ID} running={isLive} />}
    </div>
  );
}
