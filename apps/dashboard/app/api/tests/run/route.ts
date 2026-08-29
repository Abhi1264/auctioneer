import { isRunActive, runEngineTests } from "@/lib/tests/runner";
import { parseSuite } from "@/lib/tests/types";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";
export const maxDuration = 120;

export async function POST(request: Request) {
  if (isRunActive()) {
    return Response.json(
      {
        error: "A run is already in progress.",
        hint: "Wait for the current run to finish, then start another.",
      },
      { status: 409 },
    );
  }

  let suite = parseSuite(undefined);
  try {
    const body = (await request.json()) as { suite?: string };
    suite = parseSuite(body.suite);
  } catch {
    suite = parseSuite(undefined);
  }

  const encoder = new TextEncoder();
  const stream = new ReadableStream({
    start(controller) {
      const send = (payload: unknown) => {
        controller.enqueue(encoder.encode(`data: ${JSON.stringify(payload)}\n\n`));
      };

      void runEngineTests(suite, send).finally(() => {
        controller.close();
      });
    },
  });

  return new Response(stream, {
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-store",
      Connection: "keep-alive",
    },
  });
}
