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
  readdirSync,
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
 * The two clocks over a hung wait, and the order between them is the point.
 *
 * Playwright's is the one that knows anything: when it gives up it names the
 * selector, what the locator resolved to, how far the action got and how long
 * it waited. Bun's knows only that the test ran long, and when it fires it
 * takes the file's Chromium down with it — so every later test in the file
 * fails too, reporting `Target page, context or browser has been closed` and
 * naming nothing. That is what these two used to be worth 30 000 ms each, with
 * bun's starting first (TQ-0092).
 *
 * So Playwright has to run out first, and the gap has to be wide enough for the
 * error it raises to be reported. Lowering Playwright's rather than raising
 * bun's, because bun's is also what a file pays per test once its browser is
 * gone: raising it to 60s made one wedged file cost nine minutes instead of
 * four and a half, without making any single failure more legible. 20s is above
 * every deliberate wait in the suite — the longest is READY_TIMEOUT_MS, and no
 * test asks for more than 10s — so nothing that used to pass now runs out of
 * time. Both are pinned here rather than one being left to a library default,
 * since a default that moved would silently swap the order back.
 */
const TEST_TIMEOUT_MS = 30_000;
const PAGE_TIMEOUT_MS = 20_000;

/** How long to wait for a page or a browser to close before walking away from
 * it. See `closeWithin`. */
const CLOSE_TIMEOUT_MS = 5_000;

/**
 * The same, for `tq serve`, which is a process rather than a Playwright object.
 *
 * Longer than the five seconds `serve` gives its own handlers to return
 * (`internal/cli/cli.go`): equal budgets would expire together, and a server
 * that shut down correctly at the last moment would be SIGKILLed and reported
 * as stuck — the very tie between two clocks that the pair above exists to
 * avoid.
 */
const SERVER_STOP_TIMEOUT_MS = 8_000;

/**
 * The board's poll interval, from `frontend/state.ts`. The tests that prove the
 * poll runs — and the ones that prove it stands down — have to outlast it.
 */
export const POLL_INTERVAL_MS = 3000;

/** Environment that must not reach a spawned `tq`: a developer with
 * TQ_CONFIG_PATH exported must never have the suite operate on their own queue
 * (TQ-0021). */
const NEUTRALISED = ["TQ_CONFIG_PATH", "TQ_HOST", "TQ_PORT", "DEV"];

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
let binaryDir: string | undefined;
let buildFailure: string | undefined;

/**
 * Builds tq once per test file and returns its path. Building rather than
 * reusing the committed ./tq is deliberate: a stale binary would test
 * yesterday's server.
 *
 * A failure is cached exactly as hard as a success. It used to be cached not at
 * all, so a tree that does not compile paid for a full `go build` in every one
 * of a hundred tests and printed the compiler's answer a hundred times, which
 * is how a one-line syntax error reads as a hundred unrelated failures.
 */
export function tqBinary(): string {
  if (builtBinary) return builtBinary;
  if (buildFailure) {
    // The whole of it is in the first failure, and repeating it once per test
    // is what buried it. One line is enough to recognise it as the same one.
    throw new Error(`building tq failed: ${buildFailure.split("\n")[0]} (first failure has it in full)`);
  }

  const dir = mkdtempSync(join(tmpdir(), "tq-browser-bin-"));
  binaryDir = dir;
  const binary = join(dir, "tq");
  const built = Bun.spawnSync(["go", "build", "-o", binary, "./cmd/tq"], {
    cwd: REPO_ROOT,
    env: cleanEnv(),
    stdout: "pipe",
    stderr: "pipe",
    timeout: BUILD_TIMEOUT_MS,
  });
  if (built.exitCode !== 0) {
    // A build bun killed at BUILD_TIMEOUT_MS says nothing on either stream, and
    // a failure that names nothing is no better cached than uncached.
    buildFailure =
      built.stderr.toString().trim() ||
      built.stdout.toString().trim() ||
      `go build exited ${built.exitCode} without a word — killed at ${BUILD_TIMEOUT_MS}ms?`;
    throw new Error(`building tq failed:\n${buildFailure}`);
  }

  builtBinary = binary;
  return binary;
}

/**
 * Removes the binary and the directory holding it, and forgets both.
 *
 * The end of a test file is the last moment bun offers: `process.on("exit")`
 * does not fire under `bun test`, and an `afterAll` written at module scope
 * belongs to whichever file happened to load this module first rather than to
 * the run. So the binary is rebuilt once per file — 0.17s of warm `go build`
 * each — rather than once per run and never removed, which is how a directory
 * with a `tq` in it accumulated per run until the temp directory held
 * gigabytes of them (TQ-0092).
 *
 * The build *failure* is deliberately not forgotten: a tree that does not
 * compile has to be reported once for the run, not once per file.
 */
function releaseBinary(): void {
  if (binaryDir) rmSync(binaryDir, { recursive: true, force: true });
  binaryDir = undefined;
  builtBinary = undefined;
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

  /**
   * Runs the binary and fails loudly, on both streams, when it did not succeed.
   *
   * "Succeed" includes having said something: under load `Bun.spawnSync` has
   * been seen returning 0 with an empty stdout, and a caller that parses that
   * output reports a JSON syntax error rather than the command that produced no
   * JSON (TQ-0092). Every such caller passes `--json`, which is precisely the
   * flag that promises stdout is not empty, so that is the condition to check.
   */
  mustRun(...args: string[]): RunResult {
    const command = `tq ${args.join(" ")}`;
    const result = this.run(...args);
    if (result.code !== 0) {
      throw new Error(
        `${command} = ${result.code}\nstdout: ${result.stdout}\nstderr: ${result.stderr}`,
      );
    }
    if (args.includes("--json") && result.stdout.trim() === "") {
      throw new Error(`${command} exited 0 and printed nothing on stdout\nstderr: ${result.stderr}`);
    }
    return result;
  }

  /** Seeds a task via CLI; defaults to --status todo (inbox is intake, not ready). */
  add(title: string, ...flags: string[]): string {
    const created = this.mustRun("add", title, "--status", "todo", "--json", ...flags);
    return (JSON.parse(created.stdout) as { id: string }).id;
  }

  /**
   * Takes a task out of the queue by deleting its file.
   *
   * There is no `tq delete`, and there does not need to be: a task can leave a
   * queue by being deleted with the rest of a branch, or moved out of it by
   * hand, and a board with the file open in a dialog still has to cope.
   */
  remove(id: string): void {
    const dir = join(this.dir, ".tasks");
    const file = readdirSync(dir).find((name) => name.startsWith(`${id}-`) && name.endsWith(".md"));
    if (file === undefined) throw new Error(`no task file for ${id} in ${dir}`);
    rmSync(join(dir, file));
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

  /** Stops every server this project started and removes its directory — the
   * directory even when a server refused to stop, since a failure to report is
   * no reason to also leak the gigabytes this suite used to leak. */
  async cleanup(): Promise<void> {
    const servers = this.servers.splice(0);
    try {
      for (const server of servers) await server.stop();
    } finally {
      rmSync(this.dir, { recursive: true, force: true });
    }
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
  // Before anything is created: a tree that does not build must not leave a log
  // directory and two open descriptors behind for every test that tried.
  const binary = tqBinary();

  const logDir = mkdtempSync(join(tmpdir(), "tq-browser-log-"));
  const outPath = join(logDir, "stdout.log");
  const errPath = join(logDir, "stderr.log");

  const read = (path: string): string => {
    try {
      return readFileSync(path, "utf8");
    } catch {
      return "";
    }
  };

  let child: ReturnType<typeof Bun.spawn> | undefined;

  let stopped = false;
  const stop = async () => {
    if (stopped) return;
    stopped = true;
    const running = child;
    child = undefined;
    try {
      if (running) {
        // Waiting for `serve` to go is what makes its log readable to the end.
        // Waiting *unconditionally* is what let one starved server time the
        // hook out and leak the directory below (TQ-0092), so past the bound
        // the signal stops being negotiable and the directory goes regardless.
        running.kill();
        const gone = await closeWithin(
          "a server",
          () => running.exited.then(() => {}),
          SERVER_STOP_TIMEOUT_MS,
        );
        if (!gone) running.kill("SIGKILL");
      }
    } finally {
      rmSync(logDir, { recursive: true, force: true });
    }
  };

  // A server nobody ever gets a handle to is a server nobody can stop, so
  // everything from the open onwards unwinds through `stop`.
  try {
    // The descriptors belong to the child from here on. It is handed copies,
    // and nothing in this process ever writes through the originals — the logs
    // are read back by path — so they are closed immediately rather than at
    // stop time. Held until then, they outlive the child, and closing a number
    // the OS has since handed to something else closes that instead: the EBADF
    // it raised on the second of the pair is what used to abandon the removal
    // below, and is where the orphaned `tq-browser-log-*` came from (TQ-0092).
    const outFd = openSync(outPath, "w");
    const errFd = openSync(errPath, "w");
    try {
      child = Bun.spawn([binary, "serve", "--port", "0"], {
        cwd: dir,
        env: cleanEnv(),
        stdout: outFd,
        stderr: errFd,
      });
    } finally {
      closeSync(outFd);
      closeSync(errFd);
    }

    // One deadline over both waits, not one each: they are two halves of the
    // same "is it up yet", and a budget per half can add up past the test's own
    // and hand the failure back to bun, which cannot say which half it was.
    const ready = Date.now() + READY_TIMEOUT_MS;

    // The banner carries the address the listener actually got, which is the
    // only way to learn the port when it was chosen by the OS.
    const url = await waitFor(
      ready,
      () => {
        const banner = read(outPath);
        const at = banner.indexOf("http://");
        if (at < 0) return undefined;
        const rest = banner.slice(at);
        const end = rest.search(/\s/);
        return end < 0 ? undefined : rest.slice(0, end);
      },
      () =>
        `serve printed no address within ${READY_TIMEOUT_MS}ms\nstdout: ${read(outPath)}\nstderr: ${read(errPath)}`,
    );

    // And it has to actually answer before a page is pointed at it.
    await waitFor(
      ready,
      async () => {
        try {
          const response = await fetch(`${url}/api/status`);
          return response.ok ? true : undefined;
        } catch {
          return undefined;
        }
      },
      () => `server at ${url} never became ready within ${READY_TIMEOUT_MS}ms\nstderr: ${read(errPath)}`,
    );

    return { url, stderr: () => read(errPath), stop };
  } catch (error) {
    await stop();
    throw error;
  }
}

/** Polls until the probe returns something, then hands it back; past the
 * deadline it throws the message the caller built. */
async function waitFor<T>(
  deadline: number,
  probe: () => T | undefined | Promise<T | undefined>,
  message: () => string,
): Promise<T> {
  while (Date.now() < deadline) {
    const value = await probe();
    if (value !== undefined) return value;
    await Bun.sleep(20);
  }
  throw new Error(message());
}

/**
 * Waits for something to close, and stops waiting once `timeout` has passed.
 * Answers whether it closed in time, for a caller with a harder way to ask.
 *
 * Every close in the teardown can outlast the hook it runs in. Playwright asks
 * a browser to close and SIGKILLs it 30 seconds later, and under load it spends
 * all 30; a page whose renderer is wedged never finishes closing at all; a
 * server starved of CPU can sit on a SIGTERM. Either way it is the hook that
 * times out, reported against a test that had nothing to do with it — and a
 * hook bun killed is a hook that never reached the lines removing the project
 * and the logs, which is how one wedged page also became a leak (TQ-0092).
 *
 * Walking away orphans nothing. Playwright's own kill timer is still armed and
 * fires whether or not anyone is waiting on it, and bun reaps what is left of a
 * run's children when the run ends — that is the `killed N dangling processes`
 * in its output.
 */
async function closeWithin(
  what: string,
  close: () => Promise<void>,
  timeout: number,
): Promise<boolean> {
  // The rejection is handled here rather than after the race, because the loser
  // of a race is still a live promise: a rejection landing on it later would be
  // reported as an unhandled error against whichever test is running by then.
  // There is nothing it could say that matters — the thing is going away.
  const closing = Promise.resolve()
    .then(close)
    .catch(() => {});

  let timer: ReturnType<typeof setTimeout> | undefined;
  const expiry = new Promise<boolean>((resolve) => {
    timer = setTimeout(() => resolve(false), timeout);
  });
  try {
    const closed = await Promise.race([closing.then(() => true), expiry]);
    if (!closed) console.warn(`${what} did not close within ${timeout}ms; leaving it to be killed`);
    return closed;
  } finally {
    clearTimeout(timer);
  }
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
  // Tracked apart from the boards, because a project exists — with a directory
  // and, a moment later, a server — before any board is built on it.
  const projects: Project[] = [];

  beforeAll(async () => {
    browser = await launchChromium();
  });

  afterEach(async () => {
    const boards = opened.splice(0);
    const created = projects.splice(0);

    // Every page goes first, and they must: a page left open keeps polling a
    // server that is about to stop existing, which is both noise and load on
    // every test that follows. All of them before any project, because two
    // boards can share one — stopping that server with the second page still
    // open is the very thing this order exists to avoid. A page that will not
    // close in time is the one case the order cannot hold; that is a worse
    // trade than stalling the whole file behind it, but only just.
    for (const board of boards) {
      // Bounded, and its errors dropped, for the same reason: one board that
      // will not tear down must not strand the servers and temp directories of
      // the boards after it — whether it refuses by throwing, because the
      // browser is already gone, or by never answering at all.
      await closeWithin("a page", () => board.page.close(), CLOSE_TIMEOUT_MS);
    }

    let failure: unknown;
    for (const project of created) {
      try {
        await project.cleanup();
      } catch (error) {
        failure ??= error;
      }
    }
    if (failure !== undefined) throw failure;
  });

  afterAll(async () => {
    const instance = browser;
    browser = undefined;
    try {
      if (instance) await closeWithin("Chromium", () => instance.close(), CLOSE_TIMEOUT_MS);
    } finally {
      releaseBinary();
    }
  });

  /** Points a fresh page at a running server and waits for it to render. */
  const show = async (project: Project, server: Server, before?: BeforeLoad): Promise<Board> => {
    if (!browser) throw new Error("the board was opened outside a test");

    const page = await browser.newPage();
    const board = { project, server, page };
    opened.push(board);

    // Pinned rather than left to Playwright's default, so a hung wait always
    // reports itself before bun's clock can tear the page down. See the two
    // constants at the top of this file.
    page.setDefaultTimeout(PAGE_TIMEOUT_MS);
    page.setDefaultNavigationTimeout(PAGE_TIMEOUT_MS);

    await before?.(page);
    await page.goto(server.url, { waitUntil: "domcontentloaded" });
    // Any column, not a named one: the board's columns come from the project's
    // config, so a test with its own board has no `todo` to wait for.
    await page.waitForSelector(".column", { timeout: READY_TIMEOUT_MS });
    return board;
  };

  const open = async (seed?: Seed, before?: BeforeLoad): Promise<Board> => {
    // Registered the instant it exists, and before `serve` spawns anything: a
    // `seed` that throws, or a server that never comes up, used to leave a
    // directory and a child process that `afterEach` had no way to reach.
    const project = new Project();
    projects.push(project);

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

/** The centre of an element, in page coordinates, for driving the mouse. */
export async function centre(page: Page, selector: string): Promise<{ x: number; y: number }> {
  const box = await page.locator(selector).boundingBox();
  if (!box) throw new Error(`${selector} has no box to aim at`);
  return { x: box.x + box.width / 2, y: box.y + box.height / 2 };
}

/**
 * Edits one of the dialog's click-to-edit fields and lets go of it.
 *
 * Three steps rather than a `fill`, and every one of them is the behaviour
 * under test somewhere: the value reads as text until it is clicked, the
 * editor carries `<id>-edit`, and the write happens when the editor closes
 * (TQ-0069). A test that only wants the editor open stops after `openEditor`.
 */
export async function openEditor(page: Page, id: string, value: string): Promise<void> {
  await page.click(`#${id}`);
  await page.waitForSelector(`#${id}-edit`);
  await page.fill(`#${id}-edit`, value);
}

/** Opens an editor, types, and closes it with Enter — one field, written. */
export async function editField(page: Page, id: string, value: string): Promise<void> {
  await openEditor(page, id, value);
  await page.press(`#${id}-edit`, "Enter");
  await page.waitForSelector(`#${id}-edit`, { state: "detached" });
}

/**
 * The same for the description, which is a textarea: Enter is a newline there,
 * so the Save button beside it is what closes the editor.
 */
export async function editBody(page: Page, value: string): Promise<void> {
  await openEditor(page, "task-body", value);
  await page.click(".inline-actions button.primary");
  await page.waitForSelector("#task-body-edit", { state: "detached" });
}

/**
 * Chooses from one of the dialog's selects and waits for the write it starts.
 *
 * The wait is the point. A select writes the moment it is used, so a test that
 * asserted straight afterwards would be racing a PATCH that had not been sent
 * yet — and would pass or fail on how long the assertion above it happened to
 * take. Only for a change that is expected to land: a refused one sends no
 * PATCH at all, and is waited for by its toast instead.
 */
export async function choose(page: Page, id: string, value: string): Promise<void> {
  const written = page.waitForResponse(
    (response) => response.request().method() === "PATCH" && response.ok(),
  );
  await page.selectOption(`#${id}`, value);
  await written;
}

/** The IDs currently rendered in a column, top to bottom. */
export async function idsIn(page: Page, status: string): Promise<string[]> {
  return page.$$eval(`.column[data-status="${status}"] .card`, (cards) =>
    cards.map((element) => (element as HTMLElement).dataset.id ?? ""),
  );
}
