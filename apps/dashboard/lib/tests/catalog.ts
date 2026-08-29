type CatalogEntry = {
  title: string;
  meaning: string;
};

type PackageInfo = {
  label: string;
  blurb: string;
  order: number;
};

const PACKAGES: Record<string, PackageInfo> = {
  "internal/engine": {
    order: 0,
    label: "Bid engine",
    blurb: "Core rules: who may bid, what a bid is worth, and how Redis records it.",
  },
  "internal/transport/grpc": {
    order: 1,
    label: "Bid API",
    blurb: "The gRPC door clients use to create auctions and place bids.",
  },
  "internal/transport/ws": {
    order: 2,
    label: "Live updates",
    blurb: "Accepted bids are pushed to anyone watching the auction.",
  },
  "internal/config": {
    order: 3,
    label: "Startup settings",
    blurb: "The engine refuses bad configuration instead of starting half-broken.",
  },
};

const CHECKS: Record<string, CatalogEntry> = {
  TestLoadRejectsMalformedEnv: {
    title: "Rejects broken settings",
    meaning:
      "If someone sets a number as text (for example the Redis pool size), the process stops and names the bad setting. It will not start with guesswork.",
  },
  TestRedisAddrFromEnv: {
    title: "Reads the Redis address",
    meaning:
      "REDIS_ADDR wins when both address variables are set. Otherwise the first address in REDIS_ADDRS is used.",
  },
  TestServicePlaceBidValidation: {
    title: "Invalid bids never reach storage",
    meaning:
      "A bid without an auction, bid id, user, or a positive amount is rejected immediately. Nothing is written.",
  },
  "TestServicePlaceBidValidation/missing_auction_id": {
    title: "Missing auction id",
    meaning: "A bid that does not name an auction is rejected and never stored.",
  },
  "TestServicePlaceBidValidation/missing_bid_id": {
    title: "Missing bid id",
    meaning: "A bid without its own id is rejected. Ids are how the engine avoids charging the same bid twice.",
  },
  "TestServicePlaceBidValidation/missing_user_id": {
    title: "Missing user id",
    meaning: "A bid with no bidder is rejected.",
  },
  "TestServicePlaceBidValidation/zero_amount": {
    title: "Zero amount",
    meaning: "A bid of 0 is rejected.",
  },
  "TestServicePlaceBidValidation/negative_amount": {
    title: "Negative amount",
    meaning: "A negative bid is rejected.",
  },
  TestServicePlaceBidForwards: {
    title: "Valid bids are forwarded",
    meaning: "A complete, positive bid is handed to storage and marked accepted.",
  },
  TestServiceCreateAuctionValidation: {
    title: "Auctions need an id",
    meaning: "Creating an auction without an id fails before anything is stored.",
  },
  TestServiceCreateAuctionDefaultDuration: {
    title: "Default auction length",
    meaning:
      "If no end time is given, the auction is given the configured length (here, five minutes).",
  },
  TestServiceCreateAuctionKeepsEndAt: {
    title: "Keeps a stated end time",
    meaning: "If you set an end time, the engine does not overwrite it.",
  },
  TestCircuitBreakerOpensAfterThreshold: {
    title: "Opens after repeated Redis failures",
    meaning:
      "After enough Redis failures the breaker opens and new writes fail fast, instead of piling up.",
  },
  TestCircuitBreakerSuccessResets: {
    title: "A success resets the breaker",
    meaning: "One healthy write clears the failure count so a single blip does not lock the engine.",
  },
  TestCircuitBreakerRecoversAfterOpenFor: {
    title: "Recovers after a short pause",
    meaning: "Once the open interval passes, the engine tries Redis again.",
  },
  TestCircuitBreakerConcurrentAllowDuringOpen: {
    title: "Stays closed to traffic while open",
    meaning: "While the breaker is open, concurrent requests are all refused. None sneak through.",
  },
  TestDecodeCachedResult: {
    title: "Reads a cached bid result",
    meaning:
      "A stored bid result (accepted, too low, missing auction, already ended) decodes back into the same fields.",
  },
  TestRedisStoreWithMiniredis: {
    title: "Redis store (in-memory stand-in)",
    meaning:
      "The same bid rules run against a fake Redis so the suite does not need a live server.",
  },
  TestRedisStorePlaceBidIntegration: {
    title: "Redis store (live server)",
    meaning:
      "The same bid rules against a real Redis. Skipped unless REDIS_ADDR or REDIS_ADDRS is set.",
  },
  TestDispatcherBroadcastsStreamEvents: {
    title: "Stream events reach watchers",
    meaning:
      "When Redis emits an accepted bid, the dispatcher turns it into a live “bid accepted” message.",
  },
  TestPlaceBidInvalidArgument: {
    title: "Empty bid is an invalid argument",
    meaning: "gRPC PlaceBid with empty fields returns Invalid Argument, not a crash.",
  },
  TestPlaceBidBreakerUnavailable: {
    title: "Open breaker is Unavailable",
    meaning: "If Redis is known-down, PlaceBid returns Unavailable so clients can retry later.",
  },
  TestPlaceBidOverloaded: {
    title: "Too many in-flight bids",
    meaning:
      "When the in-flight slot is full, PlaceBid returns Resource Exhausted instead of queueing forever.",
  },
  TestPlaceBidAccepted: {
    title: "Accepted bid response",
    meaning: "A stored acceptance is returned with price, winner, and event id.",
  },
  TestCreateAuctionMissingID: {
    title: "Create auction without an id",
    meaning: "gRPC CreateAuction with no id returns Invalid Argument.",
  },
  TestHubRegisterBroadcastUnregister: {
    title: "Watch, notify, leave",
    meaning:
      "A client can join an auction, receive a bid message, and leave so the auction is no longer listed.",
  },
};

const STORE_CASES: Record<string, CatalogEntry> = {
  accept_and_duplicate: {
    title: "Accept, then treat a repeat as the same bid",
    meaning:
      "The first bid is accepted and becomes the price. Sending the same bid id again returns the cached result and does not raise the price.",
  },
  bid_too_low: {
    title: "Rejects a bid that is too low",
    meaning: "A bid below the current price is refused and the standing price is unchanged.",
  },
  auction_not_found: {
    title: "Unknown auction",
    meaning: "A bid on an auction that does not exist fails with auction_not_found.",
  },
  not_found_does_not_open_breaker: {
    title: "A missing auction does not trip the breaker",
    meaning:
      "Business errors (no such auction) are not treated as Redis outages. The breaker stays closed.",
  },
  ended_then_closed: {
    title: "Ended, then closed",
    meaning:
      "The first bid after the end time is auction_ended. Later bids are auction_closed.",
  },
  read_events: {
    title: "Reads the bid stream",
    meaning:
      "Accepted bids appear on the Redis stream in order. Asking again from the last id returns nothing new.",
  },
  max_bid_wins_under_concurrency: {
    title: "Highest bid wins when many land at once",
    meaning:
      "Forty bids at once still leave the standing price at the highest amount. No lost write.",
  },
};

export function packageInfo(packagePath: string): PackageInfo {
  for (const [suffix, info] of Object.entries(PACKAGES)) {
    if (packagePath.endsWith(suffix)) {
      return info;
    }
  }
  const tail = packagePath.split("/").slice(-2).join("/");
  return {
    order: 9,
    label: tail || packagePath || "Other",
    blurb: "Additional checks from the Go suite.",
  };
}

export function describeCheck(name: string): CatalogEntry {
  const exact = CHECKS[name];
  if (exact) {
    return exact;
  }

  const slash = name.indexOf("/");
  if (slash !== -1) {
    const parent = name.slice(0, slash);
    const child = name.slice(slash + 1);
    const store = STORE_CASES[child];
    if (store) {
      return store;
    }
    const parentEntry = CHECKS[parent];
    return {
      title: humanize(child),
      meaning: parentEntry?.meaning ?? "A named case inside a larger check.",
    };
  }

  return {
    title: humanize(name.replace(/^Test/, "")),
    meaning: "A check from the Go suite.",
  };
}

function humanize(raw: string): string {
  return raw
    .replace(/_/g, " ")
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .replace(/\s+/g, " ")
    .trim()
    .replace(/^\w/, (c) => c.toUpperCase());
}
