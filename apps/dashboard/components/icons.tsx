import { Check, CircleNotch, Minus, X } from "@phosphor-icons/react";

import type { CheckStatus } from "@/lib/tests/types";

export function StatusGlyph({
  status,
  className = "size-4",
}: {
  status: CheckStatus | "error";
  className?: string;
}) {
  switch (status) {
    case "passed":
      return <Check className={className} weight="bold" aria-hidden="true" />;
    case "failed":
    case "error":
      return <X className={className} weight="bold" aria-hidden="true" />;
    case "skipped":
      return <Minus className={className} weight="bold" aria-hidden="true" />;
    case "running":
      return <CircleNotch className={`${className} motion-safe:animate-spin`} weight="bold" aria-hidden="true" />;
  }
}

export function statusTone(status: CheckStatus | "error"): string {
  switch (status) {
    case "passed":
      return "text-pass";
    case "failed":
    case "error":
      return "text-fail";
    case "skipped":
      return "text-skip";
    case "running":
      return "text-running";
  }
}
