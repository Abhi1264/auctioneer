export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const HTTP_ADDR = process.env.AUCTION_HTTP_ADDR || "127.0.0.1:8080";

async function probe(path: string): Promise<"ok" | "down"> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 800);
  try {
    const host = HTTP_ADDR.startsWith(":") ? `127.0.0.1${HTTP_ADDR}` : HTTP_ADDR;
    const res = await fetch(`http://${host}${path}`, { signal: controller.signal, cache: "no-store" });
    return res.ok ? "ok" : "down";
  } catch {
    return "down";
  } finally {
    clearTimeout(timer);
  }
}

export async function GET() {
  const [health, ready] = await Promise.all([probe("/healthz"), probe("/readyz")]);
  return Response.json({ health, ready, addr: HTTP_ADDR });
}
