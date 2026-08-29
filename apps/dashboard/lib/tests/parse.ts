import { describeCheck, packageInfo } from "./catalog";
import { DEFAULT_REDIS } from "./runner-client";
import type { Check, PackageGroup, Suite, TestReport } from "./types";

type GoAction = "start" | "run" | "pause" | "cont" | "output" | "pass" | "fail" | "skip";

type GoEvent = {
  Time?: string;
  Action?: GoAction;
  Package?: string;
  Test?: string;
  Elapsed?: number;
  Output?: string;
};

export function emptyReport(suite: Suite, command: string, startedAt = new Date()): TestReport {
  return {
    id: `${startedAt.getTime()}-${suite}`,
    suite,
    command,
    startedAt: startedAt.toISOString(),
    finishedAt: null,
    status: "running",
    elapsedMs: 0,
    passed: 0,
    failed: 0,
    skipped: 0,
    running: 0,
    checks: [],
    groups: [],
    finding: "The suite is running. Results appear as each check finishes.",
    nextStep: "Leave this page open. A finding will replace this note when the run ends.",
    error: null,
  };
}

export function applyGoLine(report: TestReport, line: string): boolean {
  const trimmed = line.trim();
  if (!trimmed) {
    return false;
  }

  let event: GoEvent;
  try {
    event = JSON.parse(trimmed) as GoEvent;
  } catch {
    return false;
  }

  if (!event.Action || !event.Test) {
    return false;
  }

  const id = checkId(event.Package ?? "", event.Test);
  let check = report.checks.find((item) => item.id === id);
  let changed = false;
  if (!check) {
    const described = describeCheck(event.Test);
    check = {
      id,
      packagePath: event.Package ?? "",
      name: event.Test,
      title: described.title,
      meaning: described.meaning,
      status: "running",
      elapsedMs: 0,
      output: "",
    };
    report.checks.push(check);
    changed = true;
  }

  if (event.Action === "output" && event.Output) {
    check.output += event.Output;
  }

  if (event.Action === "pass" || event.Action === "fail" || event.Action === "skip") {
    check.status = event.Action === "pass" ? "passed" : event.Action === "fail" ? "failed" : "skipped";
    check.elapsedMs = Math.round((event.Elapsed ?? 0) * 1000);
    changed = true;
  }

  return changed;
}

export function finalizeReport(report: TestReport, finishedAt?: Date, error?: string): TestReport {
  const checks = [...report.checks].sort(compareChecks);
  const passed = checks.filter((check) => check.status === "passed").length;
  const failed = checks.filter((check) => check.status === "failed").length;
  const skipped = checks.filter((check) => check.status === "skipped").length;
  const running = checks.filter((check) => check.status === "running").length;
  const elapsedMs = checks.reduce((sum, check) => Math.max(sum, check.elapsedMs), report.elapsedMs);

  let status: TestReport["status"] = report.status;
  if (error) {
    status = "error";
  } else if (finishedAt) {
    status = failed > 0 ? "failed" : "passed";
  } else if (running > 0 || report.status === "running") {
    status = "running";
  }

  const groups = groupChecks(checks);
  const { finding, nextStep } = writeFinding({
    status,
    suite: report.suite,
    passed,
    failed,
    skipped,
    running,
    error,
    checks,
  });

  return {
    ...report,
    checks,
    groups,
    passed,
    failed,
    skipped,
    running,
    elapsedMs,
    status,
    finishedAt: finishedAt ? finishedAt.toISOString() : report.finishedAt,
    finding,
    nextStep,
    error: error ?? report.error,
  };
}

function checkId(packagePath: string, name: string): string {
  return `${packagePath}::${name}`;
}

function compareChecks(a: Check, b: Check): number {
  if (a.packagePath !== b.packagePath) {
    return a.packagePath.localeCompare(b.packagePath);
  }
  return a.name.localeCompare(b.name);
}

function groupChecks(checks: Check[]): PackageGroup[] {
  const byPath = new Map<string, PackageGroup>();
  for (const check of checks) {
    let group = byPath.get(check.packagePath);
    if (!group) {
      const info = packageInfo(check.packagePath);
      group = {
        path: check.packagePath,
        label: info.label,
        blurb: info.blurb,
        checks: [],
        passed: 0,
        failed: 0,
        skipped: 0,
        running: 0,
      };
      byPath.set(check.packagePath, group);
    }
    group.checks.push(check);
    if (check.status === "passed") group.passed += 1;
    if (check.status === "failed") group.failed += 1;
    if (check.status === "skipped") group.skipped += 1;
    if (check.status === "running") group.running += 1;
  }
  return [...byPath.values()].sort(
    (a, b) => packageInfo(a.path).order - packageInfo(b.path).order || a.label.localeCompare(b.label),
  );
}

function writeFinding(input: {
  status: TestReport["status"];
  suite: Suite;
  passed: number;
  failed: number;
  skipped: number;
  running: number;
  error?: string;
  checks: Check[];
}): { finding: string; nextStep: string } {
  if (input.error) {
    return {
      finding: input.error,
      nextStep: "Fix the problem named above, then run the suite again.",
    };
  }

  if (input.status === "running") {
    const done = input.passed + input.failed + input.skipped;
    return {
      finding:
        done === 0
          ? "The suite has started. Individual checks will appear as Go reports them."
          : `${done} checks have finished. ${input.running} still running.`,
      nextStep: "Wait for the run to finish. Failed checks, if any, will open with the exact error.",
    };
  }

  if (input.failed > 0) {
    const first = input.checks.find((check) => check.status === "failed");
    const named = first ? ` The first failure is “${first.title}.”` : "";
    return {
      finding: `${input.failed} ${plural(input.failed, "check")} failed.${named} Until those pass, do not treat the engine as healthy.`,
      nextStep: "Open each failed check for the Go error, fix the cause, then run again.",
    };
  }

  if (input.skipped > 0) {
    const liveSkip = input.checks.some((check) => check.name.includes("Integration"));
    if (input.suite === "unit" && liveSkip) {
      return {
        finding: `All ${input.passed} unit checks passed. ${input.skipped} live-Redis ${plural(input.skipped, "check")} ${input.skipped === 1 ? "was" : "were"} skipped because no Redis address was set. That skip is expected for a unit run.`,
        nextStep:
          `Start Redis on ${DEFAULT_REDIS} and choose Include Live Redis if you want the same rules proven against a real server.`,
      };
    }
    return {
      finding: `All runnable checks passed. ${input.skipped} ${plural(input.skipped, "check")} skipped.`,
      nextStep: "Read the skipped items. They usually name the missing dependency.",
    };
  }

  return {
    finding: `Every check passed (${input.passed}). The engine rejected bad input, accepted valid bids, and kept Redis, the bid API, and live updates behaving as designed.`,
    nextStep: "No action needed. Run again after you change engine code.",
  };
}

function plural(count: number, word: string): string {
  return count === 1 ? word : `${word}s`;
}
