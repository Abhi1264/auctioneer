"use client";

import { useId, useState } from "react";

import { CaretDown } from "@phosphor-icons/react";

import { StatusGlyph, statusTone } from "@/components/icons";
import { formatDuration, statusLabel } from "@/lib/tests/format";
import { matchesFilter, nestChecks, nodeVisible } from "@/lib/tests/tree";
import type { Check, PackageGroup, ViewFilter } from "@/lib/tests/types";

export function CheckGroups({
  groups,
  filter,
}: {
  groups: PackageGroup[];
  filter: ViewFilter;
}) {
  const visible = groups
    .map((group) => ({
      ...group,
      nodes: nestChecks(group.checks).filter((node) => nodeVisible(node, filter)),
    }))
    .filter((group) => group.nodes.length > 0);

  if (visible.length === 0) {
    return (
      <p className="px-5 py-10 text-muted sm:px-8">
        No checks match this filter. Choose All to see the full report.
      </p>
    );
  }

  return (
    <div className="divide-y divide-rule">
      {visible.map((group) => (
        <section key={group.path} className="px-5 py-6 sm:px-8" aria-labelledby={`pkg-${group.path}`}>
          <div className="flex flex-wrap items-end justify-between gap-3">
            <div className="min-w-0">
              <h3
                id={`pkg-${group.path}`}
                className="font-serif text-xl font-semibold text-pretty text-ink"
                translate="no"
              >
                {group.label}
              </h3>
              <p className="mt-1 max-w-prose text-sm text-pretty text-muted">{group.blurb}</p>
            </div>
            <p className="tabular text-sm text-muted">
              {group.failed > 0
                ? `${group.failed} failed`
                : group.running > 0
                  ? `${group.running} running`
                  : group.skipped > 0
                    ? `${group.passed} passed · ${group.skipped} skipped`
                    : `${group.passed} passed`}
            </p>
          </div>
          <ol className="mt-5 space-y-2">
            {group.nodes.map((node) => (
              <li key={node.check.id}>
                <CheckRow
                  check={node.check}
                  childrenChecks={node.children.filter((child) => matchesFilter(child, filter))}
                />
              </li>
            ))}
          </ol>
        </section>
      ))}
    </div>
  );
}

function CheckRow({
  check,
  childrenChecks,
}: {
  check: Check;
  childrenChecks: Check[];
}) {
  const failed = check.status === "failed" || childrenChecks.some((child) => child.status === "failed");
  const [openedByUser, setOpenedByUser] = useState<boolean | null>(null);
  const open = openedByUser ?? failed;
  const panelId = useId();

  return (
    <article className="rounded-sm border border-rule bg-paper/40">
      <button
        type="button"
        className="flex w-full cursor-pointer items-start gap-3 px-3 py-3 text-left min-h-11 hover:bg-paper/80"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => setOpenedByUser((value) => !(value ?? failed))}
      >
        <span className={`mt-0.5 ${statusTone(check.status)}`} aria-hidden="true">
          <StatusGlyph status={check.status} />
        </span>
        <span className="min-w-0 flex-1">
          <span className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
            <span className="font-bold text-ink">{check.title}</span>
            <span className="tabular text-sm text-muted">{formatDuration(check.elapsedMs)}</span>
          </span>
          <span className="mt-1 block text-sm text-muted">
            <span className="sr-only">{statusLabel(check.status)}. </span>
            {check.meaning}
          </span>
        </span>
        <CaretDown
          className={`mt-1 size-4 shrink-0 text-muted transition-transform duration-200 ease-out ${open ? "rotate-180" : ""}`}
          aria-hidden="true"
        />
      </button>
      {open ? (
        <div id={panelId} className="border-t border-rule px-3 py-3">
          <p className="font-mono text-xs text-muted" translate="no">
            {check.name}
          </p>
          {check.output.trim() ? (
            <pre className="mt-3 max-h-72 overflow-auto whitespace-pre-wrap wrap-break-word bg-paper px-3 py-3 font-mono text-xs text-ink">
              {check.output.trim()}
            </pre>
          ) : (
            <p className="mt-2 text-sm text-muted">
              {check.status === "running"
                ? "Waiting for Go to finish this check…"
                : "No extra log. The result is the status above."}
            </p>
          )}
          {childrenChecks.length > 0 ? (
            <ol className="mt-4 space-y-2 border-t border-rule pt-3">
              {childrenChecks.map((child) => (
                <li key={child.id} className="flex items-start gap-3">
                  <span className={`mt-0.5 ${statusTone(child.status)}`} aria-hidden="true">
                    <StatusGlyph status={child.status} />
                  </span>
                  <div className="min-w-0">
                    <p className="font-bold text-ink">
                      {child.title}
                      <span className="sr-only"> {statusLabel(child.status)}</span>
                    </p>
                    <p className="text-sm text-muted">{child.meaning}</p>
                    {child.status === "failed" && child.output.trim() ? (
                      <pre className="mt-2 max-h-56 overflow-auto whitespace-pre-wrap wrap-break-word bg-paper px-3 py-2 font-mono text-xs">
                        {child.output.trim()}
                      </pre>
                    ) : null}
                  </div>
                </li>
              ))}
            </ol>
          ) : null}
        </div>
      ) : null}
    </article>
  );
}

