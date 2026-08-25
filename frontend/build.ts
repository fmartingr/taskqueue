/**
 * Frontend build: bundles app.ts and copies the static files into public/.
 *
 * Bun is the only JavaScript toolchain, and it is build-time only — the Go
 * binary embeds public/ and serves it without any runtime dependency.
 *
 *   bun run frontend/build.ts            build once
 *   bun run frontend/build.ts --watch    rebuild on change (used by `make dev`)
 */

import { watch } from "node:fs";

const SOURCE_DIR = "frontend";
const OUTPUT_DIR = "public";
const COPIES = ["index.html", "style.css", "icon.png"];

async function build(): Promise<void> {
  const started = performance.now();

  const result = await Bun.build({
    entrypoints: [`${SOURCE_DIR}/app.ts`],
    outdir: OUTPUT_DIR,
    target: "browser",
    naming: "[dir]/[name].js",
    minify: false,
    sourcemap: "none",
  });

  if (!result.success) {
    for (const log of result.logs) console.error(log);
    throw new AggregateError(result.logs, "frontend build failed");
  }

  for (const name of COPIES) {
    await Bun.write(`${OUTPUT_DIR}/${name}`, Bun.file(`${SOURCE_DIR}/${name}`));
  }

  const elapsed = Math.round(performance.now() - started);
  console.log(`built ${OUTPUT_DIR}/ (${[...COPIES, "app.js"].join(", ")}) in ${elapsed}ms`);
}

await build();

if (process.argv.includes("--watch")) {
  console.log(`watching ${SOURCE_DIR}/ …`);
  let pending: ReturnType<typeof setTimeout> | undefined;

  watch(SOURCE_DIR, { recursive: true }, () => {
    clearTimeout(pending);
    pending = setTimeout(() => {
      build().catch((error: unknown) => console.error(error));
    }, 50);
  });
}
