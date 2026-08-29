const numberFormat = new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 });

const dateFormat = new Intl.DateTimeFormat(undefined, {
  dateStyle: "medium",
  timeStyle: "short",
});

export function formatCount(value: number): string {
  return numberFormat.format(value);
}

export function formatWhen(iso: string | null): string {
  if (!iso) return "—";
  return dateFormat.format(new Date(iso));
}

export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) {
    return "0 ms";
  }
  if (ms < 1000) {
    return `${numberFormat.format(Math.round(ms))}\u00a0ms`;
  }
  const seconds = ms / 1000;
  if (seconds < 60) {
    const rounded = seconds >= 10 ? Math.round(seconds) : Math.round(seconds * 10) / 10;
    return `${rounded.toString()}\u00a0s`;
  }
  const minutes = Math.floor(seconds / 60);
  const rest = Math.round(seconds % 60);
  return `${minutes}\u00a0m ${rest}\u00a0s`;
}

export function statusLabel(status: "passed" | "failed" | "skipped" | "running" | "error"): string {
  switch (status) {
    case "passed":
      return "Passed";
    case "failed":
      return "Failed";
    case "skipped":
      return "Skipped";
    case "running":
      return "Running";
    case "error":
      return "Could not run";
  }
}
