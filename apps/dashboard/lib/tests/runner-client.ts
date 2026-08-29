import type { Suite } from "./types";

export const DEFAULT_REDIS = "127.0.0.1:6379";
export const GO_TEST_ARGS = ["test", "-json", "-count=1", "./..."] as const;

export function commandFor(suite: Suite): string {
  const env = suite === "redis" ? `REDIS_ADDR=${DEFAULT_REDIS} ` : "";
  return `${env}CGO_ENABLED=0 go ${GO_TEST_ARGS.join(" ")}`;
}
