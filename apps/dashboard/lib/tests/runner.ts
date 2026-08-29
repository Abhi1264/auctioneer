import { spawn } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import { createInterface } from "node:readline";

import { applyGoLine, emptyReport, finalizeReport } from "./parse";
import { findApiRoot } from "./paths";
import { commandFor, DEFAULT_REDIS, GO_TEST_ARGS } from "./runner-client";
import type { RunnerEvent, Suite, TestReport } from "./types";

const REPORT_PATH = path.join(os.tmpdir(), "auctioneer-last-report.json");

let latest: TestReport | null = null;
let running = false;

function remember(report: TestReport, disk = false) {
  latest = report;
  if (!disk) return;
  try {
    writeFileSync(REPORT_PATH, JSON.stringify(report));
  } catch {
    // Keep the in-memory copy even if the disk write fails.
  }
}

export function getLatestReport(): TestReport | null {
  if (latest) return latest;
  try {
    latest = JSON.parse(readFileSync(REPORT_PATH, "utf8")) as TestReport;
    return latest;
  } catch {
    return null;
  }
}

export function isRunActive(): boolean {
  return running;
}

export async function runEngineTests(
  suite: Suite,
  emit: (event: RunnerEvent) => void,
): Promise<TestReport> {
  if (running) {
    throw new Error("A run is already in progress.");
  }
  running = true;

  try {
    const apiRoot = findApiRoot();
    let report = emptyReport(suite, commandFor(suite));
    remember(report, true);
    emit({ type: "started", report });

    report = await spawnGoTest(apiRoot, suite, report, (next) => {
      remember(next);
      emit({ type: "updated", report: next });
    });

    remember(report, true);
    emit({ type: "finished", report });
    return report;
  } catch (error) {
    const message = error instanceof Error ? error.message : "The test run failed.";
    const failed = finalizeReport(
      latest ?? emptyReport(suite, commandFor(suite)),
      new Date(),
      message,
    );
    remember(failed, true);
    emit({ type: "error", message, hint: hintFor(message), report: failed });
    return failed;
  } finally {
    running = false;
  }
}

function spawnGoTest(
  cwd: string,
  suite: Suite,
  seed: TestReport,
  onUpdate: (report: TestReport) => void,
): Promise<TestReport> {
  return new Promise((resolve) => {
    const env: NodeJS.ProcessEnv = { ...process.env, CGO_ENABLED: "0" };
    if (suite === "redis") {
      env.REDIS_ADDR = process.env.REDIS_ADDR || DEFAULT_REDIS;
      env.REDIS_ADDRS = process.env.REDIS_ADDRS || env.REDIS_ADDR;
    } else {
      delete env.REDIS_ADDR;
      delete env.REDIS_ADDRS;
    }

    const child = spawn("go", [...GO_TEST_ARGS], {
      cwd,
      env,
      stdio: ["ignore", "pipe", "pipe"],
    });

    const report = seed;
    let stderr = "";
    let settled = false;
    let lastEmit = 0;
    let dirty = false;

    const publish = () => {
      lastEmit = Date.now();
      dirty = false;
      onUpdate(finalizeReport(report));
    };

    const finish = (error?: string) => {
      if (settled) return;
      settled = true;
      resolve(finalizeReport(report, new Date(), error));
    };

    child.on("error", (error) => {
      finish(
        error.message.includes("ENOENT")
          ? "Go is not installed or not on PATH. Install Go, then run the suite again."
          : error.message,
      );
    });

    const onLine = (line: string) => {
      if (applyGoLine(report, line)) dirty = true;
      if (dirty && Date.now() - lastEmit >= 200) publish();
    };

    if (child.stdout) {
      const out = createInterface({ input: child.stdout });
      out.on("line", onLine);
    }

    if (child.stderr) {
      const err = createInterface({ input: child.stderr });
      err.on("line", (line) => {
        stderr += `${line}\n`;
        onLine(line);
      });
    }

    child.on("close", (code) => {
      if (code === 0 || report.checks.length > 0) {
        finish();
        return;
      }
      finish(
        stderr.trim() ||
          `go test exited with code ${code ?? "unknown"} before any checks were reported.`,
      );
    });
  });
}

function hintFor(message: string): string {
  if (message.includes("PATH") || message.includes("Go is not")) {
    return "Install the Go toolchain and confirm `go version` works in this terminal.";
  }
  if (message.includes("apps/api")) {
    return "Start the dashboard from the auctioneer repo, or set AUCTION_API_DIR to apps/api.";
  }
  if (message.includes("already in progress")) {
    return "Wait for the current run to finish, then start another.";
  }
  return "Read the error, fix the cause, and run again.";
}
