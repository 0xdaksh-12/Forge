import { useEffect, useRef, useState } from "react";
import { api } from "../api/client";
import type { LogLine } from "../api/client";

interface Props {
  jobId: number;
  running: boolean;
}

export function LogViewer({ jobId, running }: Props) {
  const [lines, setLines] = useState<LogLine[]>([]);
  const bottomRef = useRef<HTMLDivElement>(null);
  const viewerRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);

  useEffect(() => {
    // Initial load/reset is handled by the 'key' prop in the parent component,
    // which unmounts/remounts LogViewer when jobId changes.

    let es: EventSource | null = null;
    let cancelled = false;

    if (running) {
      es = new EventSource(api.events.jobLogsUrl(jobId));
      es.onmessage = (e) => {
        if (cancelled) return;
        try {
          const parsed = JSON.parse(e.data as string) as {
            seq: number;
            stream: string;
            text: string;
          };
          setLines((prev) => [
            ...prev,
            {
              ID: parsed.seq,
              JobID: jobId,
              Seq: parsed.seq,
              Stream: parsed.stream as "stdout" | "stderr",
              Text: parsed.text,
              Timestamp: new Date().toISOString(),
            },
          ]);
        } catch {
          /* ignore malformed event */
        }
      };
      es.addEventListener("done", () => es?.close());
    } else {
      api.jobs
        .logs(jobId)
        .then((logs) => {
          if (!cancelled) setLines(logs ?? []);
        })
        .catch(() => {});
    }

    return () => {
      cancelled = true;
      es?.close();
    };
  }, [jobId, running]);

  useEffect(() => {
    if (autoScroll) bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [lines, autoScroll]);

  const handleScroll = () => {
    const el = viewerRef.current;
    if (!el) return;
    setAutoScroll(el.scrollHeight - el.scrollTop - el.clientHeight < 40);
  };

  return (
    <div className="log-viewer" ref={viewerRef} onScroll={handleScroll}>
      {lines.length === 0 && (
        <span style={{ color: "var(--text-muted)" }}>Waiting for logs…</span>
      )}
      {lines.map((l) => (
        <div key={l.Seq} className="log-line">
          <span className="log-seq">{l.Seq}</span>
          <span
            className={`log-text log-stream-${l.Stream}${l.Text.startsWith("--- ") ? " step-header" : ""}`}
          >
            {l.Text}
          </span>
        </div>
      ))}
      <div ref={bottomRef} />
    </div>
  );
}
