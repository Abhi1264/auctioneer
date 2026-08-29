import { StatusGlyph } from "@/components/icons";
import { formatCount, formatDuration, formatWhen, statusLabel } from "@/lib/tests/format";
import type { TestReport } from "@/lib/tests/types";

const stampClass = {
  passed: "text-pass border-pass",
  failed: "text-fail border-fail",
  running: "text-running border-running",
  error: "text-fail border-fail",
} as const;

export function VerdictPanel({ report }: { report: TestReport }) {
  const stamp = statusLabel(report.status);
  const tone = stampClass[report.status];

  return (
    <section
      aria-labelledby="verdict-heading"
      className="fade-in grid gap-8 border-b border-rule px-5 py-8 sm:px-8 lg:grid-cols-[auto_minmax(0,1fr)] lg:items-start"
    >
      <div className="flex justify-center lg:justify-start">
        <p
          className={`stamp min-h-20 min-w-44 border-[3px] px-5 py-3 text-center font-serif text-2xl font-semibold uppercase tracking-[0.18em] ${tone}`}
          aria-hidden="true"
        >
          {stamp}
        </p>
      </div>

      <div className="min-w-0">
        <h2 id="verdict-heading" className="font-serif text-2xl font-semibold text-pretty text-ink">
          {report.status === "running" ? "Inspection in Progress" : "Finding"}
        </h2>
        <p className="mt-3 max-w-prose text-pretty text-ink">{report.finding}</p>
        <p className="mt-3 max-w-prose text-pretty text-muted">{report.nextStep}</p>

        <dl className="mt-6 grid grid-cols-2 gap-x-6 gap-y-3 text-sm sm:grid-cols-4">
          <Stat label="Passed" value={formatCount(report.passed)} tone="text-pass" />
          <Stat label="Failed" value={formatCount(report.failed)} tone="text-fail" />
          <Stat label="Skipped" value={formatCount(report.skipped)} tone="text-skip" />
          <Stat
            label="Duration"
            value={report.status === "running" ? "Live" : formatDuration(report.elapsedMs)}
          />
        </dl>

        <p className="mt-4 text-sm text-muted">
          <span className="sr-only">Status: {statusLabel(report.status)}. </span>
          {report.finishedAt ? "Finished " : "Started "}
          <time dateTime={report.finishedAt ?? report.startedAt} suppressHydrationWarning>
            {formatWhen(report.finishedAt ?? report.startedAt)}
          </time>
          {report.status === "running" ? (
            <span className="ml-2 inline-flex items-center gap-1 text-running">
              <StatusGlyph status="running" />
              Running…
            </span>
          ) : null}
        </p>
      </div>
    </section>
  );
}

function Stat({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone?: string;
}) {
  return (
    <div>
      <dt className="text-muted">{label}</dt>
      <dd className={`tabular text-lg font-bold ${tone ?? "text-ink"}`}>{value}</dd>
    </div>
  );
}
