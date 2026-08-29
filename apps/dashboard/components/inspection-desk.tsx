"use client";

import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { CaretDown, Printer } from "@phosphor-icons/react";

import { CheckGroups } from "@/components/check-groups";
import { StatusGlyph } from "@/components/icons";
import { VerdictPanel } from "@/components/verdict-panel";
import { commandFor, DEFAULT_REDIS } from "@/lib/tests/runner-client";
import type { RunnerEvent, Suite, TestReport, ViewFilter } from "@/lib/tests/types";

type EngineStatus = {
  health: "ok" | "down";
  ready: "ok" | "down";
  addr: string;
};

export function InspectionDesk({
  initialReport,
  initialSuite = "unit",
  initialView = "all",
}: {
  initialReport: TestReport | null;
  initialSuite?: Suite;
  initialView?: ViewFilter;
}) {
  const router = useRouter();
  const [suite, setSuite] = useState<Suite>(initialSuite);
  const [view, setView] = useState<ViewFilter>(initialView);

  const [report, setReport] = useState<TestReport | null>(initialReport);
  const [busy, setBusy] = useState(initialReport?.status === "running");
  const [runError, setRunError] = useState<string | null>(null);
  const [engine, setEngine] = useState<EngineStatus | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = () => {
      void fetch("/api/engine/status", { cache: "no-store" })
        .then((res) => res.json())
        .then((data: EngineStatus) => {
          if (!cancelled) setEngine(data);
        })
        .catch(() => undefined);
    };
    load();
    const id = setInterval(load, 8000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  const replaceQuery = useCallback(
    (next: { suite?: Suite; view?: ViewFilter }) => {
      const nextSuite = next.suite ?? suite;
      const nextView = next.view ?? view;
      if (next.suite !== undefined) setSuite(next.suite);
      if (next.view !== undefined) setView(next.view);
      const query = new URLSearchParams();
      if (nextSuite === "redis") query.set("suite", nextSuite);
      if (nextView !== "all") query.set("view", nextView);
      const qs = query.toString();
      router.replace(qs ? `/?${qs}` : "/", { scroll: false });
    },
    [router, suite, view],
  );

  const run = useCallback(async () => {
    if (busy) return;
    setBusy(true);
    setRunError(null);
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    try {
      const res = await fetch("/api/tests/run", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ suite }),
        signal: controller.signal,
      });

      if (res.status === 409) {
        const body = (await res.json()) as { error: string; hint: string };
        setRunError(`${body.error} ${body.hint}`);
        setBusy(false);
        return;
      }

      if (!res.ok || !res.body) {
        setRunError("The dashboard could not start go test. Confirm the API app is in this repo and Go is installed.");
        setBusy(false);
        return;
      }

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";

      while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const chunks = buffer.split("\n\n");
        buffer = chunks.pop() ?? "";
        for (const chunk of chunks) {
          const line = chunk.split("\n").find((item) => item.startsWith("data: "));
          if (!line) continue;
          const event = JSON.parse(line.slice(6)) as RunnerEvent;
          if (event.type === "error") {
            setRunError(`${event.message} ${event.hint}`);
            if (event.report) setReport(event.report);
            continue;
          }
          setReport(event.report);
        }
      }
    } catch (error) {
      if (error instanceof DOMException && error.name === "AbortError") {
        return;
      }
      setRunError("The connection to the test runner dropped. Run again. If it keeps happening, check that Go is on PATH.");
    } finally {
      setBusy(false);
    }
  }, [busy, suite]);

  const live = useMemo(() => {
    if (!report) return "No inspection yet.";
    if (report.status === "running") {
      return `${report.passed + report.failed + report.skipped} finished, ${report.running} running.`;
    }
    return report.finding;
  }, [report]);

  return (
    <>
      <a
        href="#report"
        className="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-20 focus:bg-lot focus:px-3 focus:py-2"
      >
        Skip to Report
      </a>

      <header className="no-print border-b border-rule pt-[env(safe-area-inset-top)]">
        <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 px-5 py-6 sm:px-8">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div>
              <p className="font-mono text-xs uppercase tracking-[0.22em] text-muted">Lot · Engine</p>
              <h1 className="mt-1 font-serif text-3xl font-semibold tracking-tight text-pretty sm:text-4xl">
                Auctioneer
              </h1>
              <p className="mt-2 max-w-xl text-pretty text-muted">
                Run the real Go suite in <code className="font-mono text-sm text-ink">apps/api</code> and read what it
                proved — in English, not raw test names.
              </p>
            </div>
            <EngineBadge status={engine} />
          </div>

          <form
            className="flex flex-col gap-4"
            onSubmit={(event) => {
              event.preventDefault();
              void run();
            }}
          >
            <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
              <button
                type="submit"
                disabled={busy}
                className="inline-flex min-h-11 cursor-pointer items-center justify-center gap-2 bg-ledger px-5 font-bold text-on-ledger transition-colors duration-200 ease-out hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {busy ? <StatusGlyph status="running" /> : null}
                {busy ? "Running…" : "Run Engine Tests"}
              </button>

              <label className="inline-flex min-h-11 cursor-pointer items-center gap-3 text-ink">
                <input
                  type="checkbox"
                  name="includeRedis"
                  autoComplete="off"
                  className="size-5 accent-ledger"
                  checked={suite === "redis"}
                  disabled={busy}
                  onChange={(event) => replaceQuery({ suite: event.target.checked ? "redis" : "unit" })}
                />
                Include Live Redis
              </label>

              <button
                type="button"
                className="inline-flex min-h-11 cursor-pointer items-center gap-2 px-3 text-ink hover:text-ledger disabled:cursor-not-allowed disabled:opacity-50"
                disabled={!report || report.status === "running"}
                onClick={() => window.print()}
              >
                <Printer className="size-4" aria-hidden="true" />
                Print Report
              </button>
            </div>

            <p className="font-mono text-xs break-all text-muted" translate="no">
              {commandFor(suite)}
            </p>
            <p className="text-sm text-muted">
              {suite === "redis"
                ? `Sets REDIS_ADDR to ${DEFAULT_REDIS} so the live Redis check runs. Start Redis first.`
                : "Unit run. The live Redis check is skipped unless you include it."}
            </p>
          </form>

          {runError ? (
            <p role="alert" className="max-w-prose text-pretty text-fail">
              {runError}
            </p>
          ) : null}
        </div>
      </header>

      <main id="report" className="mx-auto w-full max-w-5xl flex-1 px-5 py-8 sm:px-8 scroll-mt-6">
        <p className="sr-only" aria-live="polite">
          {live}
        </p>

        {!report ? (
          <section className="lot-card border border-rule px-5 py-10 sm:px-8">
            <h2 className="font-serif text-2xl font-semibold text-pretty">Not Inspected Yet</h2>
            <p className="mt-3 max-w-prose text-pretty text-muted">
              This is not sample data. “Run Engine Tests” executes{" "}
              <span className="font-mono text-sm text-ink" translate="no">
                {commandFor("unit")}
              </span>{" "}
              in the API app, then turns each result into a short finding anyone can read.
            </p>
          </section>
        ) : (
          <article className="lot-card border border-rule">
            <VerdictPanel report={report} />

            <div className="no-print flex flex-wrap gap-2 border-b border-rule px-5 py-3 sm:px-8">
              {(
                [
                  ["all", "All"],
                  ["failed", "Failed"],
                  ["skipped", "Skipped"],
                  ["passed", "Passed"],
                ] as const
              ).map(([value, label]) => {
                const selected = view === value;
                return (
                  <button
                    key={value}
                    type="button"
                    className={`min-h-11 cursor-pointer px-3 text-sm font-bold transition-colors duration-200 ease-out ${
                      selected ? "bg-ink text-lot" : "text-ink hover:bg-paper"
                    }`}
                    aria-pressed={selected}
                    onClick={() => replaceQuery({ view: value })}
                  >
                    {label}
                    {value !== "all" ? (
                      <span className="tabular ml-2 font-normal opacity-80">
                        {value === "failed" ? report.failed : value === "skipped" ? report.skipped : report.passed}
                      </span>
                    ) : null}
                  </button>
                );
              })}
            </div>

            <CheckGroups groups={report.groups} filter={view} />
          </article>
        )}

        <HowToRead />
      </main>
    </>
  );
}

function EngineBadge({ status }: { status: EngineStatus | null }) {
  if (!status) {
    return (
      <p className="font-mono text-xs uppercase tracking-[0.18em] text-muted">Engine · checking…</p>
    );
  }

  const live = status.health === "ok";
  const ready = status.ready === "ok";
  return (
    <p className="text-right text-sm">
      <span className="font-mono text-xs uppercase tracking-[0.18em] text-muted">Engine</span>
      <span className={`mt-1 block font-bold ${live ? "text-pass" : "text-muted"}`}>
        {live ? "Process up" : "Not running"}
        {live ? (
          <span className="mt-0.5 block font-normal text-muted">
            Redis {ready ? "ready" : "not ready"} · {status.addr}
          </span>
        ) : (
          <span className="mt-0.5 block font-normal text-muted">Optional. Tests do not need it.</span>
        )}
      </span>
    </p>
  );
}

function HowToRead() {
  return (
    <details className="no-print mt-8 group">
      <summary className="flex min-h-11 cursor-pointer list-none items-center justify-between gap-3 font-bold text-ink">
        How to Read This Report
        <CaretDown className="size-4 text-muted transition-transform duration-200 group-open:rotate-180" aria-hidden="true" />
      </summary>
      <div className="mt-3 max-w-prose space-y-3 text-pretty text-muted">
        <p>
          <strong className="text-ink">Passed</strong> means that check behaved as the suite requires.{" "}
          <strong className="text-ink">Failed</strong> means the engine did something the suite forbids — open the check
          for the exact Go error.
        </p>
        <p>
          <strong className="text-ink">Skipped</strong> is not a failure. The live Redis check skips on a unit run
          because no Redis address is set. Include Live Redis only after Redis is listening.
        </p>
        <p>
          Counts use the real <span className="font-mono text-sm text-ink">go test -json</span> stream. There is no
          fixture data behind this page.
        </p>
      </div>
    </details>
  );
}

