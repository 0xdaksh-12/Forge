import { useEffect, useState } from "react";
import { api } from "../api/client";
import type { Pipeline } from "../api/client";
import { reltime } from "../components/utils";
import { Link } from "react-router-dom";

interface CreateForm {
  Name: string;
  RepoURL: string;
  GitHubRepo: string;
  WebhookSecret: string;
  DefaultBranch: string;
}

const EMPTY: CreateForm = {
  Name: "",
  RepoURL: "",
  GitHubRepo: "",
  WebhookSecret: "",
  DefaultBranch: "main",
};

export function Pipelines() {
  const [pipelines, setPipelines] = useState<Pipeline[]>([]);
  const [showModal, setShowModal] = useState(false);
  const [form, setForm] = useState<CreateForm>(EMPTY);
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState("");

  const load = () =>
    api.pipelines
      .list()
      .then((p) => setPipelines(p ?? []))
      .catch(() => {});

  useEffect(() => {
    load();
  }, []);

  const handleCreate = async () => {
    setSaving(true);
    setErr("");
    try {
      await api.pipelines.create(form);
      setShowModal(false);
      setForm(EMPTY);
      load();
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "Unknown error");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm("Delete this pipeline and all its builds?")) return;
    await api.pipelines.delete(id);
    load();
  };

  const handleTrigger = async (id: number) => {
    try {
      await api.webhooks.trigger(id);
    } catch (e: unknown) {
      alert(e instanceof Error ? e.message : String(e));
    }
  };

  return (
    <>
      <div className="page-header">
        <div>
          <h1 className="page-title">Pipelines</h1>
          <p className="page-subtitle">
            Registered repositories and their webhook config
          </p>
        </div>
        <button className="btn btn-primary" onClick={() => setShowModal(true)}>
          <span className="material-icons material-icons-inline">add</span> New
          Pipeline
        </button>
      </div>

      <div className="page-body">
        <div className="card">
          {pipelines.length === 0 ? (
            <div className="empty-state">
              <div className="empty-icon">
                <span className="material-icons" style={{ fontSize: "48px" }}>
                  settings
                </span>
              </div>
              <div className="empty-title">No pipelines yet</div>
              <div className="empty-desc">
                Click{" "}
                <strong>
                  <span className="material-icons material-icons-inline">
                    add
                  </span>{" "}
                  New Pipeline
                </strong>{" "}
                to register a repository.
              </div>
            </div>
          ) : (
            <div className="build-list">
              {pipelines.map((p) => (
                <div
                  className="build-row"
                  key={p.ID}
                  style={{ cursor: "default" }}
                >
                  <div className="build-number">#{p.ID}</div>
                  <div className="build-info">
                    <Link
                      to={`/pipelines/${p.ID}`}
                      className="build-commit"
                      style={{ color: "var(--brand)" }}
                    >
                      {p.Name}
                    </Link>
                    <div className="build-meta">
                      <span className="build-meta-tag">
                        <span className="material-icons material-icons-inline">
                          link
                        </span>{" "}
                        {p.RepoURL}
                      </span>

                      {p.GitHubRepo && (
                        <span className="build-meta-tag">
                          <span className="material-icons material-icons-inline">
                            inventory_2
                          </span>{" "}
                          {p.GitHubRepo}
                        </span>
                      )}

                      <span className="build-meta-tag">
                        <span className="material-icons material-icons-inline">
                          account_tree
                        </span>{" "}
                        {p.DefaultBranch}
                      </span>

                      <span className="build-meta-tag text-muted">
                        {reltime(p.CreatedAt)}
                      </span>
                    </div>
                  </div>
                  <div className="flex gap-8">
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={() => handleTrigger(p.ID)}
                    >
                      <span className="material-icons material-icons-inline">
                        play_arrow
                      </span>{" "}
                      Run
                    </button>

                    <button
                      className="btn btn-danger btn-sm"
                      onClick={() => handleDelete(p.ID)}
                    >
                      <span className="material-icons material-icons-inline">
                        delete
                      </span>
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Webhook instructions */}
        <div className="card" style={{ marginTop: 20 }}>
          <div className="card-header">
            <div className="card-title">GitHub Webhook Setup</div>
          </div>
          <div
            className="card-body text-sm"
            style={{ color: "var(--text-secondary)", lineHeight: 1.8 }}
          >
            <p>Point your GitHub repository webhook at:</p>
            <code
              style={{
                display: "block",
                margin: "10px 0",
                padding: "10px 14px",
                background: "var(--bg-elevated)",
                borderRadius: "var(--radius-md)",
              }}
            >
              POST {window.location.origin}/api/webhooks/github
            </code>
            <p>
              Set <strong>Content type</strong> to <code>application/json</code>
              , choose events <strong>push</strong> and{" "}
              <strong>pull_request</strong>, and enter your webhook secret.
            </p>
          </div>
        </div>
      </div>

      {/* Create modal */}
      {showModal && (
        <div className="modal-backdrop" onClick={() => setShowModal(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-title">New Pipeline</div>

            {err && (
              <div
                style={{
                  color: "var(--error)",
                  fontSize: 13,
                  marginBottom: 12,
                }}
              >
                {err}
              </div>
            )}

            {(Object.keys(EMPTY) as (keyof CreateForm)[]).map((key) => (
              <div className="form-group" key={key}>
                <label className="form-label">
                  {key === "GitHubRepo"
                    ? "GitHub Repo (owner/repo)"
                    : key === "RepoURL"
                      ? "Clone URL"
                      : key === "WebhookSecret"
                        ? "Webhook Secret"
                        : key === "DefaultBranch"
                          ? "Default Branch"
                          : "Name"}
                </label>
                <input
                  className="form-input"
                  placeholder={
                    key === "GitHubRepo"
                      ? "octocat/hello-world"
                      : key === "RepoURL"
                        ? "https://github.com/octocat/hello-world.git"
                        : key === "WebhookSecret"
                          ? "super-secret"
                          : key === "DefaultBranch"
                            ? "main"
                            : "my-service"
                  }
                  value={form[key]}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, [key]: e.target.value }))
                  }
                />
              </div>
            ))}

            <div className="modal-actions">
              <button
                className="btn btn-ghost"
                onClick={() => setShowModal(false)}
              >
                Cancel
              </button>
              <button
                className="btn btn-primary"
                onClick={handleCreate}
                disabled={saving || !form.Name || !form.RepoURL}
              >
                {saving ? "Creating…" : "Create"}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
