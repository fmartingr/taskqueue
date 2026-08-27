/**
 * Browser test harness.
 *
 * These tests drive the real board in a real Chromium, against the real `tq`
 * binary serving a real project directory. They exist because the parts of
 * the board that matter most — native drag and drop, `<dialog>`, focus
 * and blur, the poll — are exactly the parts a simulated DOM gets wrong.
 *
 * The shape mirrors `internal/integration/harness_test.go`, because the hard
 * parts are the same ones: build the binary once, give every test its own
 * project with its own marker so discovery cannot climb out, start the server
 * on port 0 so the suite is parallel-safe, read the address from the banner,
 * and print the server's stderr when readiness fails rather than time out in
 * silence.
 *
 * Run with: make test-browser
 */

import { afterAll, afterEach, beforeAll, setDefaultTimeout } from "bun:test";
import { chromium, type Browser, type Page } from "playwright-core";
import {
  closeSync,
  mkdirSync,
  mkdtempSync,
  openSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const REPO_ROOT = resolve(import.meta.dir, "..");

/** How long to wait for `go build`, the server banner and readiness. */
const BUILD_TIMEOUT_MS = 120_000;
const READY_TIMEOUT_MS = 15_000;

/**
 * Bun's default is 5s, which the poll tests outlast on purpose — and a test bun
 * times out takes the file's Chromium down with it, so every later test in the
 * file fails too. Raising it keeps a real failure legible.
 */
const TEST_TIMEOUT_MS = 30_000;

/**
 * The board's poll interval, from `frontend/state.ts`. The tests that prove the
 * poll runs — and the ones that prove it stands down — have to outlast it.
 */
export const POLL_INTERVAL_MS = 3000;

/** Environment that must not reach a spawned `tq`: a developer with TQ_DIR
 * exported must never have the suite operate on their own queue (TQ-0021). */
const NEUTRALISED = ["TQ_DIR", "TQ_HOST", "TQ_PORT", "DEV"];

function cleanEnv(): Record<string, string> {
  const env: Record<string, string> = {};
  for (const [key, value] of Object.entries(process.env)) {
    if (value !== undefined && !NEUTRALISED.includes(key)) env[key] = value;
  }
  return env;
}

// ── Chromium ────────────────────────────────────────────────────

const INSTALL_HINT =
  "Install it with:  make browser-install   (bunx playwright-core install chromium)\n" +
  "  or point PLAYWRIGHT_BROWSERS_PATH at a cache that already has one.";

/**
 * Launches the Chromium playwright-core installed.
 *
 * Deliberately no `executablePath`: playwright-core resolves the browser from
 * the same registry that downloaded it, so the layout inside a
 * `chromium-<revision>` directory stays its business. This file used to spell
 * those paths out and got them wrong — the macOS entries were migrated to
 * Chrome for Testing and the Linux one was left as `chrome-linux/chrome`, which
 * passes on a Mac and cannot pass on Linux (TQ-0077). Letting the library
 * answer also lets it pick the headless shell when it prefers one.
 *
 * The only thing worth adding is the error: Playwright's own says
 * `npx playwright install`, which is not how this repository installs it.
 */
export async function launchChromium(): Promise<Browser> {
  try {
    return await chromium.launch({ headless: true });
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`Could not launch Chromium.\n  ${INSTALL_HINT}\n\n${message}`);
  }
}

// ── The binary ──────────────────────────────────────────────────

let builtBinary: string | undefined;

/**
 * Builds tq once per run and returns its path. Building rather than reusing the
 * committed ./tq is deliberate: a stale binary would test yesterday's server.
 */
export function tqBinary(): string {
  if (builtBinary) return builtBinary;

  const dir = mkdtempSync(join(tmpdir(), "tq-browser-bin-"));
  const binary = join(dir, "tq");
  const built = Bun.spawnSync(["go", "build", "-o", binary, "./cmd/tq"], {
    cwd: REPO_ROOT,
    env: cleanEnv(),
    stdout: "pipe",
    stderr: "pipe",
    timeout: BUILD_TIMEOUT_MS,
  });
  if (built.exitCode !== 0) {
    throw new Error(`building tq failed:\n${built.stderr.toString()}`);
  }

  builtBinary = binary;
  return binary;
}

// ── Projects and servers ────────────────────────────────────────

export interface RunResult {
  code: number;
  stdout: string;
  stderr: string;
}

/**
 * A project is a temp directory carrying its own `.taskqueue.yaml`, so
 * discovery resolves inside it and cannot climb out into whatever happens to
 * sit above the temp directory.
 */
export class Project {
  readonly dir: string;
  private servers: Server[] = [];

  constructor() {
    this.dir = mkdtempSync(join(tmpdir(), "tq-browser-"));
    writeFileSync(join(this.dir, ".taskqueue.yaml"), "version: 1\npath: .tasks\n");
    mkdirSync(join(this.dir, ".tasks"), { recursive: true });
  }

  /** Runs the binary inside the project and returns what a shell would see. */
  run(...args: string[]): RunResult {
    const done = Bun.spawnSync([tqBinary(), ...args], {
      cwd: this.dir,
      env: cleanEnv(),
      stdout: "pipe",
      stderr: "pipe",
    });
    return {
      code: done.exitCode,
      stdout: done.stdout.toString(),
      stderr: done.stderr.toString(),
    };
  }

  /** Runs the binary and fails loudly, on both streams, when it did not succeed. */
  mustRun(...args: string[]): RunResult {
    const result = this.run(...args);
    if (result.code !== 0) {
      throw new Error(
        `tq ${args.join(" ")} = ${result.code}\nstdout: ${result.stdout}\nstderr: ${result.stderr}`,
      );
    }
    return result;
  }

  /** Seeds a task via CLI; defaults to --status todo (inbox is intake, not ready). */
  add(title: string, ...flags: string[]): string {
    const created = this.mustRun("add", title, "--status", "todo", "--json", ...flags);
    return (JSON.parse(created.stdout) as { id: string }).id;
  }

  /** Reads the tasks straight from the API, which is what "the move reached the
   * server" means: the assertion never goes through the page's own state. */
  async tasks(server: Server): Promise<Task[]> {
    const response = await fetch(`${server.url}/api/tasks`);
    if (!response.ok) throw new Error(`GET /api/tasks = ${response.status}`);
    return (await response.json()) as Task[];
  }

  async serve(): Promise<Server> {
    const server = await startServer(this.dir);
    this.servers.push(server);
    return server;
  }

  async cleanup(): Promise<void> {
    for (const server of this.servers) await server.stop();
    this.servers = [];
    rmSync(this.dir, { recursive: true, force: true });
  }
}

export interface Task {
  id: string;
  title: string;
  status: string;
  priority?: string;
  assignee?: string;
  labels?: string[];
  depends_on?: string[];
  body?: string;
  created?: string;
  updated?: string;
}

export interface Server {
  url: string;
  /** Everything the server has logged so far, for a failure message. */
  stderr(): string;
  stop(): Promise<void>;
}

/**
 * Starts `tq serve` on a port the OS picks and waits until it answers.
 *
 * Its two streams go to files rather than pipes, which is not a detail: `tq
 * serve` logs every request on stderr, and a Go program that writes to a broken
 * pipe on fd 1 or 2 is killed by SIGPIPE. A reader that stops reading — a
 * cancelled stream, a collected one — therefore kills the server mid-test, and
 * that is exactly the flake this suite started with. A file cannot break, and
 * reading it back is what `stderr()` does.
 */
async function startServer(dir: string): Promise<Server> {
  const logDir = mkdtempSync(join(tmpdir(), "tq-browser-log-"));
  const outPath = join(logDir, "stdout.log");
  const errPath = join(logDir, "stderr.log");
  const outFd = openSync(outPath, "w");
  const errFd = openSync(errPath, "w");

  const read = (path: string): string => {
    try {
      return readFileSync(path, "utf8");
    } catch {
      return "";
    }
  };

  const child = Bun.spawn([tqBinary(), "serve", "--port", "0"], {
    cwd: dir,
    env: cleanEnv(),
    stdout: outFd,
    stderr: errFd,
  });

  let stopped = false;
  const stop = async () => {
    if (stopped) return;
    stopped = true;
    child.kill();
    await child.exited;
    closeSync(outFd);
    closeSync(errFd);
    rmSync(logDir, { recursive: true, force: true });
  };

  // The banner carries the address the listener actually got, which is the only
  // way to learn the port when it was chosen by the OS.
  const url = await waitFor(
    () => {
      const banner = read(outPath);
      const at = banner.indexOf("http://");
      if (at < 0) return undefined;
      const rest = banner.slice(at);
      const end = rest.search(/\s/);
      return end < 0 ? undefined : rest.slice(0, end);
    },
    () => `serve printed no address within ${READY_TIMEOUT_MS}ms\nstdout: ${read(outPath)}\nstderr: ${read(errPath)}`,
    stop,
  );

  // And it has to actually answer before a page is pointed at it.
  await waitFor(
    async () => {
      try {
        const response = await fetch(`${url}/api/status`);
        return response.ok ? true : undefined;
      } catch {
        return undefined;
      }
    },
    () => `server at ${url} never became ready\nstderr: ${read(errPath)}`,
    stop,
  );

  return { url, stderr: () => read(errPath), stop };
}

/** Polls until the probe returns something, then hands it back; on timeout it
 * stops the server and throws the message the caller built. */
async function waitFor<T>(
  probe: () => T | undefined | Promise<T | undefined>,
  message: () => string,
  onTimeout?: () => Promise<void>,
): Promise<T> {
  const deadline = Date.now() + READY_TIMEOUT_MS;
  while (Date.now() < deadline) {
    const value = await probe();
    if (value !== undefined) return value;
    await Bun.sleep(20);
  }
  if (onTimeout) await onTimeout();
  throw new Error(message());
}

// ── The board ───────────────────────────────────────────────────

/** One test's project, its server and a page showing its board. */
export interface Board {
  project: Project;
  server: Server;
  page: Page;
}

/** Seeds a project through the CLI before its server starts. */
export type Seed = (project: Project) => void;

/**
 * Installs everything a browser test file needs — a Chromium, a timeout that
 * outlasts the poll, and cleanup of every board a test opened — and returns the
 * one function those tests call.
 *
 * Call it once at the top level of a test file.
 */
/**
 * Runs against a page before it is pointed at the board, for a test that needs
 * the page configured before the first render — intercepting a request the
 * board makes on load, say, which is too late to do after goto.
 */
export type BeforeLoad = (page: Page) => Promise<void>;

/** The call a test file makes to get a board of its own. */
export type OpenBoard = (seed?: Seed, before?: BeforeLoad) => Promise<Board>;

/**
 * What useBoard hands back: the call above, plus `another` for the tests about
 * a change reaching more than one board.
 */
export interface BoardOpener extends OpenBoard {
  /**
   * A second board on a server that is already running: same project, same
   * server, its own page. It is what proves that one scan serves every
   * connected board rather than each browser fetching for itself.
   *
   * Its own browser context, because Playwright will not make a second page in
   * the one `browser.newPage()` created — and a context of its own is closer to
   * two people looking at the same board anyway.
   */
  another(board: Board): Promise<Board>;
}

export function useBoard(): BoardOpener {
  setDefaultTimeout(TEST_TIMEOUT_MS);

  // One Chromium per test file, held in this closure rather than in a module
  // variable: nothing another file does can reach it.
  let browser: Browser | undefined;
  const opened: Board[] = [];

  beforeAll(async () => {
    browser = await launchChromium();
  });

  afterEach(async () => {
    const boards = opened.splice(0);

    // Every page goes first, and they must: a page left open keeps polling a
    // server that is about to stop existing, which is both noise and load on
    // every test that follows. All of them before any project, because two
    // boards can share one — stopping that server with the second page still
    // open is the very thing this order exists to avoid.
    for (const board of boards) {
      // One board failing to tear down must not strand the servers and temp
      // directories of the boards after it.
      try {
        await board.page.close();
      } catch {
        // The browser is already gone; the project still has to be cleaned up.
      }
    }

    for (const project of new Set(boards.map((board) => board.project))) {
      await project.cleanup();
    }
  });

  afterAll(async () => {
    await browser?.close();
    browser = undefined;
  });

  /** Points a fresh page at a running server and waits for it to render. */
  const show = async (project: Project, server: Server, before?: BeforeLoad): Promise<Board> => {
    if (!browser) throw new Error("the board was opened outside a test");

    const page = await browser.newPage();
    const board = { project, server, page };
    opened.push(board);

    await before?.(page);
    await page.goto(server.url, { waitUntil: "domcontentloaded" });
    // Any column, not a named one: the board's columns come from the project's
    // config, so a test with its own board has no `todo` to wait for.
    await page.waitForSelector(".column", { timeout: READY_TIMEOUT_MS });
    return board;
  };

  const open = async (seed?: Seed, before?: BeforeLoad): Promise<Board> => {
    const project = new Project();
    // Seeding before the server starts means the first render already has the
    // tasks, so no test has to wait a poll for its own fixtures.
    seed?.(project);

    return show(project, await project.serve(), before);
  };

  return Object.assign(open, {
    another: (board: Board) => show(board.project, board.server),
  });
}

/** The card for a task, wherever it currently is. */
export const card = (id: string) => `.card[data-id="${id}"]`;

/** The card for a task, but only while it sits in a given column. */
export const cardIn = (status: string, id: string) => `.column[data-status="${status}"] ${card(id)}`;

/** The IDs currently rendered in a column, top to bottom. */
export async function idsIn(page: Page, status: string): Promise<string[]> {
  return page.$$eval(`.column[data-status="${status}"] .card`, (cards) =>
    cards.map((element) => (element as HTMLElement).dataset.id ?? ""),
  );
}
