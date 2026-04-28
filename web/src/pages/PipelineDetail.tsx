import { useEffect, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { api } from "../api/client";
import type { Pipeline, Build, Secret } from "../api/client";
import { reltime } from "../components/utils";

export function PipelineDetail() {
  const { id } = useParams();
  const pipelineId = Number(id);

  const [pipeline, setPipeline] = useState<Pipeline | null>(null);
  const [builds, setBuilds] = useState<Build[]>([]);
  const [secrets, setSecrets] = useState<Secret[]>([]);
  const [activeTab, setActiveTab] = useState<"builds" | "secrets">("builds");
  const [loading, setLoading] = useState(true);

  // Secret form
  const [secretName, setSecretName] = useState("");
  const [secretValue, setSecretValue] = useState("");
  const [savingSecret, setSavingSecret] = useState(false);

  useEffect(() => {
    if (!pipelineId) return;
    
    Promise.all([
      api.pipelines.get(pipelineId),
      api.pipelines.secrets.list(pipelineId),
    ])
      .then(([res, secs]) => {
        setPipeline(res.pipeline);
        setBuilds(res.builds || []);
        setSecrets(secs || []);
      })
      .catch((e) => console.error("Failed to load pipeline details", e))
      .finally(() => setLoading(false));
  }, [pipelineId]);

  const handleTrigger = async () => {
    try {
      await api.webhooks.trigger(pipelineId);
      // Reload builds after triggering
      const res = await api.pipelines.get(pipelineId);
      setBuilds(res.builds || []);
    } catch (e: unknown) {
      alert(e instanceof Error ? e.message : String(e));
    }
  };

  const handleSaveSecret = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!secretName || !secretValue) return;
    
    setSavingSecret(true);
    try {
      await api.pipelines.secrets.put(pipelineId, secretName, secretValue);
      setSecretName("");
      setSecretValue("");
      const secs = await api.pipelines.secrets.list(pipelineId);
      setSecrets(secs || []);
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : "Failed to save secret");
    } finally {
      setSavingSecret(false);
    }
  };

  const handleDeleteSecret = async (name: string) => {
    if (!confirm(`Delete secret ${name}?`)) return;
    try {
      await api.pipelines.secrets.delete(pipelineId, name);
      setSecrets((s) => s.filter((sec) => sec.Name !== name));
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : "Failed to delete secret");
    }
  };

  if (loading) {
    return <div className="page-body">Loading...</div>;
  }

  if (!pipeline) {
    return <div className="page-body">Pipeline not found.</div>;
  }

  return (
    <>
      <div className="page-header">
        <div>
          <div className="flex gap-8" style={{ alignItems: "center", marginBottom: 8 }}>
            <Link to="/pipelines" className="text-muted" style={{ textDecoration: "none" }}>
              <span className="material-icons material-icons-inline">arrow_back</span>
            </Link>
            <h1 className="page-title" style={{ margin: 0 }}>{pipeline.Name}</h1>
          </div>
          <p className="page-subtitle">
            <span className="material-icons material-icons-inline">link</span> {pipeline.RepoURL}
          </p>
        </div>
        <button className="btn btn-primary" onClick={handleTrigger}>
          <span className="material-icons material-icons-inline">play_arrow</span> Trigger Build
        </button>
      </div>

      <div className="tabs" style={{ marginBottom: 20, borderBottom: "1px solid var(--border)", display: "flex", gap: 20 }}>
        <button 
          className={`tab ${activeTab === "builds" ? "active" : ""}`}
          onClick={() => setActiveTab("builds")}
          style={{ 
            background: "none", border: "none", padding: "10px 0", cursor: "pointer",
            borderBottom: activeTab === "builds" ? "2px solid var(--brand)" : "2px solid transparent",
            color: activeTab === "builds" ? "var(--text)" : "var(--text-secondary)",
            fontWeight: activeTab === "builds" ? 600 : 400
          }}
        >
          Recent Builds
        </button>
        <button 
          className={`tab ${activeTab === "secrets" ? "active" : ""}`}
          onClick={() => setActiveTab("secrets")}
          style={{ 
            background: "none", border: "none", padding: "10px 0", cursor: "pointer",
            borderBottom: activeTab === "secrets" ? "2px solid var(--brand)" : "2px solid transparent",
            color: activeTab === "secrets" ? "var(--text)" : "var(--text-secondary)",
            fontWeight: activeTab === "secrets" ? 600 : 400
          }}
        >
          Secrets Vault
        </button>
      </div>

      <div className="page-body">
        {activeTab === "builds" && (
          <div className="card">
            {builds.length === 0 ? (
              <div className="empty-state">
                <div className="empty-title">No builds yet</div>
              </div>
            ) : (
              <div className="build-list">
                {builds.map((b) => (
                  <Link to={`/builds/${b.ID}`} className="build-row" key={b.ID}>
                    <div className="build-number">#{b.ID}</div>
                    <div className="build-info">
                      <div className="build-commit">{b.CommitMsg || "Manual Trigger"}</div>
                      <div className="build-meta">
                        <span className="build-meta-tag">
                          <span className="material-icons material-icons-inline">commit</span>
                          {b.CommitSHA.substring(0, 7)}
                        </span>
                        <span className="build-meta-tag">
                          <span className="material-icons material-icons-inline">person</span>
                          {b.AuthorName || "Unknown"}
                        </span>
                        <span className="build-meta-tag text-muted">
                          {reltime(b.StartedAt || "")}
                        </span>
                      </div>
                    </div>
                    <div className={`status-badge status-${b.Status}`}>
                      {b.Status}
                    </div>
                  </Link>
                ))}
              </div>
            )}
          </div>
        )}

        {activeTab === "secrets" && (
          <div style={{ display: "flex", gap: 20, alignItems: "flex-start" }}>
            <div className="card" style={{ flex: 1 }}>
              <div className="card-header">
                <div className="card-title">Pipeline Secrets</div>
              </div>
              <div className="card-body">
                {secrets.length === 0 ? (
                  <p className="text-muted" style={{ fontSize: 14 }}>No secrets added yet.</p>
                ) : (
                  <div className="build-list" style={{ marginTop: 0 }}>
                    {secrets.map(s => (
                      <div className="build-row" key={s.ID} style={{ padding: "12px 16px", cursor: "default" }}>
                        <div className="build-info">
                          <div style={{ fontWeight: 600, fontFamily: "var(--font-mono)", fontSize: 13 }}>
                            {s.Name}
                          </div>
                          <div className="text-muted" style={{ fontSize: 12, marginTop: 4 }}>
                            Updated {reltime(s.UpdatedAt)}
                          </div>
                        </div>
                        <button 
                          className="btn btn-ghost btn-sm" 
                          style={{ color: "var(--error)" }}
                          onClick={() => handleDeleteSecret(s.Name)}
                        >
                          <span className="material-icons material-icons-inline">delete</span>
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>

            <div className="card" style={{ width: 350 }}>
              <div className="card-header">
                <div className="card-title">Add Secret</div>
              </div>
              <div className="card-body">
                <form onSubmit={handleSaveSecret}>
                  <div className="form-group">
                    <label className="form-label">Name</label>
                    <input 
                      className="form-input" 
                      placeholder="e.g. AWS_ACCESS_KEY" 
                      value={secretName}
                      onChange={e => setSecretName(e.target.value)}
                      pattern="[A-Za-z0-9_]+"
                      title="Only alphanumeric characters and underscores allowed"
                      required
                    />
                  </div>
                  <div className="form-group">
                    <label className="form-label">Value</label>
                    <textarea 
                      className="form-input" 
                      placeholder="Secure value..." 
                      value={secretValue}
                      onChange={e => setSecretValue(e.target.value)}
                      rows={4}
                      required
                    />
                  </div>
                  <button type="submit" className="btn btn-primary" style={{ width: "100%" }} disabled={savingSecret || !secretName || !secretValue}>
                    {savingSecret ? "Saving..." : "Save Secret"}
                  </button>
                  <p className="text-muted" style={{ fontSize: 12, marginTop: 12, lineHeight: 1.5 }}>
                    <span className="material-icons material-icons-inline" style={{ fontSize: 14 }}>lock</span>
                    Secrets are encrypted at rest using AES-256-GCM. Once saved, the plaintext value cannot be viewed again.
                  </p>
                </form>
              </div>
            </div>
          </div>
        )}
      </div>
    </>
  );
}
