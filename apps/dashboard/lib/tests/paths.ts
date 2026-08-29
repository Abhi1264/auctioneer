import { existsSync } from "node:fs";
import path from "node:path";

export function findApiRoot(): string {
  if (process.env.AUCTION_API_DIR) {
    return path.resolve(process.env.AUCTION_API_DIR);
  }

  const candidates = [
    path.resolve(process.cwd(), "apps/api"),
    path.resolve(process.cwd(), "../api"),
    path.resolve(process.cwd(), "../../apps/api"),
  ];

  for (const candidate of candidates) {
    if (existsSync(path.join(candidate, "go.mod"))) {
      return candidate;
    }
  }

  throw new Error(
    "Could not find apps/api. Run the dashboard from the monorepo, or set AUCTION_API_DIR.",
  );
}
