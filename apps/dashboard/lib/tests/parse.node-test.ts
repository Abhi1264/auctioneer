import assert from "node:assert/strict";
import { test } from "node:test";

import { applyGoLine, emptyReport, finalizeReport } from "./parse";

test("builds a readable report from go test JSON", () => {
  let report = emptyReport("unit", "go test -json ./...");
  const lines = [
    { Action: "run", Package: "github.com/Abhi1264/auctioneer/apps/api/internal/engine", Test: "TestServicePlaceBidForwards" },
    {
      Action: "pass",
      Package: "github.com/Abhi1264/auctioneer/apps/api/internal/engine",
      Test: "TestServicePlaceBidForwards",
      Elapsed: 0.012,
    },
    { Action: "run", Package: "github.com/Abhi1264/auctioneer/apps/api/internal/engine", Test: "TestRedisStorePlaceBidIntegration" },
    {
      Action: "output",
      Package: "github.com/Abhi1264/auctioneer/apps/api/internal/engine",
      Test: "TestRedisStorePlaceBidIntegration",
      Output: "--- SKIP: TestRedisStorePlaceBidIntegration (0.00s)\n",
    },
    {
      Action: "skip",
      Package: "github.com/Abhi1264/auctioneer/apps/api/internal/engine",
      Test: "TestRedisStorePlaceBidIntegration",
      Elapsed: 0,
    },
  ];

  for (const line of lines) {
    applyGoLine(report, JSON.stringify(line));
  }
  report = finalizeReport(report, new Date("2026-08-29T16:00:00.000Z"));

  assert.equal(report.passed, 1);
  assert.equal(report.skipped, 1);
  assert.equal(report.failed, 0);
  assert.equal(report.status, "passed");
  assert.match(report.finding, /unit checks passed/i);
  const forwarded = report.checks.find((check) => check.name === "TestServicePlaceBidForwards");
  assert.equal(forwarded?.title, "Valid bids are forwarded");
  assert.equal(report.groups[0]?.label, "Bid engine");
});

test("failed checks become a failing finding", () => {
  let report = emptyReport("unit", "go test");
  applyGoLine(
    report,
    JSON.stringify({
      Action: "fail",
      Package: "github.com/Abhi1264/auctioneer/apps/api/internal/engine",
      Test: "TestCircuitBreakerOpensAfterThreshold",
      Elapsed: 0.02,
    }),
  );
  report = finalizeReport(report, new Date());
  assert.equal(report.status, "failed");
  assert.match(report.finding, /failed/i);
  assert.match(report.nextStep, /Run again/i);
});
