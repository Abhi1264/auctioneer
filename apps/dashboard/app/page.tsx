export default function Home() {
  return (
    <main className="flex flex-1 flex-col items-start justify-center gap-3 px-8 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Auctioneer</h1>
      <p className="max-w-md text-sm text-zinc-600 dark:text-zinc-400">
        Dashboard stub. The auction engine lives in{" "}
        <code className="font-mono text-xs">apps/api</code>.
      </p>
    </main>
  );
}
