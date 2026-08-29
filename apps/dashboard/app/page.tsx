import { InspectionDesk } from "@/components/inspection-desk";
import { getLatestReport } from "@/lib/tests/runner";
import { parseSuite, type ViewFilter } from "@/lib/tests/types";

export const dynamic = "force-dynamic";

export default async function Home({
  searchParams,
}: {
  searchParams: Promise<{ suite?: string; view?: string }>;
}) {
  const params = await searchParams;
  const view: ViewFilter =
    params.view === "failed" || params.view === "skipped" || params.view === "passed"
      ? params.view
      : "all";

  return (
    <InspectionDesk
      initialReport={getLatestReport()}
      initialSuite={parseSuite(params.suite)}
      initialView={view}
    />
  );
}
