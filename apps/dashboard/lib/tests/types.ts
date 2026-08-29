export type CheckStatus = "running" | "passed" | "failed" | "skipped";

export type Suite = "unit" | "redis";

export type ViewFilter = "all" | "failed" | "skipped" | "passed";

export function parseSuite(value: unknown): Suite {
  return value === "redis" ? "redis" : "unit";
}

export type Check = {
  id: string;
  packagePath: string;
  name: string;
  title: string;
  meaning: string;
  status: CheckStatus;
  elapsedMs: number;
  output: string;
};

export type PackageGroup = {
  path: string;
  label: string;
  blurb: string;
  checks: Check[];
  passed: number;
  failed: number;
  skipped: number;
  running: number;
};

export type TestReport = {
  id: string;
  suite: Suite;
  command: string;
  startedAt: string;
  finishedAt: string | null;
  status: "running" | "passed" | "failed" | "error";
  elapsedMs: number;
  passed: number;
  failed: number;
  skipped: number;
  running: number;
  checks: Check[];
  groups: PackageGroup[];
  finding: string;
  nextStep: string;
  error: string | null;
};

export type RunnerEvent =
  | { type: "started"; report: TestReport }
  | { type: "updated"; report: TestReport }
  | { type: "finished"; report: TestReport }
  | { type: "error"; message: string; hint: string; report?: TestReport };
