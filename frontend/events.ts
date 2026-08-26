/**
 * The board's live connection to the server.
 *
 * `GET /api/events` is a server-sent event stream: the server reads the task
 * directory and the project marker on a ticker and says when either moved, and
 * the board refetches. That is the whole protocol — the frames carry a
 * fingerprint, never tasks and never configuration — so there is one
 * serialization path (the REST endpoints) rather than two that can drift apart.
 *
 * The frame's *name* is the one thing it does say: `tasks` and `config` send
 * the board to different endpoints, so they cannot be the same signal.
 *
 * The poll in state.ts stays as the fallback for when this is unavailable, so
 * the board is never silently stale.
 */

/** Where the reconnect delay starts, and where it stops growing. */
export const RECONNECT_MIN_MS = 500;
export const RECONNECT_MAX_MS = 15_000;

/**
 * How long to wait before the next attempt. Doubling, capped: a server that
 * went away for a moment is back almost at once, and one that is gone for the
 * afternoon is asked about four times a minute rather than twice a second.
 */
export function backoffDelay(attempt: number): number {
  if (attempt <= 0) return RECONNECT_MIN_MS;
  const grown = RECONNECT_MIN_MS * 2 ** attempt;
  // 2 ** big is Infinity long before it is a useful number.
  return Number.isFinite(grown) ? Math.min(grown, RECONNECT_MAX_MS) : RECONNECT_MAX_MS;
}

/**
 * How long the board waits to hear anything at all before deciding the stream
 * is dead. The server sends a `ping` well inside this, so silence for longer
 * means the connection went half-open — asleep, resumed, or dropped by
 * something in between without ever telling either end.
 */
export const SILENCE_TIMEOUT_MS = 65_000;

export interface StreamHandlers {
  /** Something in the task directory moved; refetch. */
  onTasks(): void;
  /**
   * `.taskqueue.yaml` moved; refetch the configuration (TQ-0034).
   *
   * Its own signal rather than a second reason to call onTasks, because it
   * sends the board to a different endpoint: the labels, the columns and the
   * priorities are what changed, and the listing may well be identical.
   */
  onConfig(): void;
  /** The server could not read the queue, and says why. */
  onScanFailed(message: string): void;
  /** The stream is up, or has gone down and is being retried. */
  onConnected(connected: boolean): void;
}

/**
 * Opens the stream and keeps it open, returning the function that stops it.
 *
 * The reconnect is written out rather than left to EventSource's own, which
 * retries on a delay the page cannot see or control — and the board needs to
 * know it is disconnected, because that is when the fallback poll matters.
 */
export function connectEvents(handlers: StreamHandlers, url = "/api/events"): () => void {
  let source: EventSource | null = null;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let watchdog: ReturnType<typeof setTimeout> | undefined;
  let attempt = 0;
  let stopped = false;

  const drop = (): void => {
    clearTimeout(watchdog);
    source?.close();
    source = null;
    handlers.onConnected(false);
    if (stopped) return;
    timer = setTimeout(open, backoffDelay(attempt));
    attempt += 1;
  };

  // Any frame is proof the connection is alive, including the server's ping.
  const heard = (): void => {
    clearTimeout(watchdog);
    watchdog = setTimeout(drop, SILENCE_TIMEOUT_MS);
  };

  const open = (): void => {
    if (stopped) return;
    source = new EventSource(url);
    heard();

    source.addEventListener("open", () => {
      attempt = 0;
      heard();
      handlers.onConnected(true);
    });

    source.addEventListener("ping", heard);

    source.addEventListener("tasks", () => {
      heard();
      handlers.onTasks();
    });

    source.addEventListener("config", () => {
      heard();
      handlers.onConfig();
    });

    source.addEventListener("scan-failed", (message) => {
      heard();
      handlers.onScanFailed((message as MessageEvent<string>).data);
    });

    // EventSource dispatches its own failures here, which is why the server's
    // read errors are called "scan-failed" and not "error".
    source.addEventListener("error", drop);
  };

  open();

  return () => {
    stopped = true;
    clearTimeout(timer);
    clearTimeout(watchdog);
    source?.close();
    source = null;
  };
}
