/**
 * Frontend build: bundles the Vue app and copies the static files into public/.
 *
 * Bun is the only JavaScript toolchain, and it is build-time only — the Go
 * binary embeds public/ and serves it without any runtime dependency. Vue is a
 * runtime dependency of the *source*, not of the output: it is bundled into
 * app.js, so nothing under node_modules/ is ever served.
 *
 *   bun run frontend/build.ts            build once
 *   bun run frontend/build.ts --watch    rebuild on change (used by `make dev`)
 */

import { pluginVue3 } from "bun-plugin-vue3";
import { watch } from "node:fs";

const SOURCE_DIR = "frontend";
// The built frontend lives beside the package that embeds it: go:embed cannot
// reach outside its own directory.
const OUTPUT_DIR = "internal/web/public";
// favicon.png is a downscaled derivative of the full-size artwork in
// frontend/icon.png, which stays out of the binary and is used by the README.
// Bun has no image pipeline and this project takes no JavaScript dependencies,
// so it is generated once and committed. To regenerate after changing the
// artwork:
//
//   magick frontend/icon.png -resize 256x256 -strip -colors 256 \
//     -define png:compression-level=9 frontend/favicon.png
const COPIES = ["index.html", "style.css", "favicon.png"];

/**
 * The bundle keeps the name index.html asks for and go:embed lists, so the
 * entrypoint can be called whatever reads best. Everything else in the naming
 * is Bun's default.
 */
const OUTPUT_NAME = "app.js";

async function build(): Promise<void> {
  const started = performance.now();

  const result = await Bun.build({
    entrypoints: [`${SOURCE_DIR}/main.ts`],
    outdir: OUTPUT_DIR,
    target: "browser",
    naming: { entry: OUTPUT_NAME },
    // Vue reads three flags at bundle time. Defining them here is what drops
    // the Options API and the devtools hooks from the output; leaving them
    // undefined ships both and warns in the console at start-up.
    //
    // NODE_ENV is the fourth: Vue's bundler build gates its whole development
    // half — the component-tree warnings, the prop validators — on it, and Bun
    // does not assume production for a library it is only bundling. Without it
    // the binary would ship Vue's development runtime.
    define: {
      __VUE_OPTIONS_API__: "false",
      __VUE_PROD_DEVTOOLS__: "false",
      __VUE_PROD_HYDRATION_MISMATCH_DETAILS__: "false",
      "process.env.NODE_ENV": JSON.stringify("production"),
    },
    // isProduction strips the SFC compiler's development-only helpers (hot
    // reload hooks, __file annotations), which is what keeps the output
    // deterministic and free of paths from the machine that built it.
    plugins: [pluginVue3({ isProduction: true })],
    // Unminified on purpose: the output is committed, so a readable diff is
    // worth more than the bytes, and the staleness gate compares it verbatim.
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
  console.log(`built ${OUTPUT_DIR}/ (${[...COPIES, OUTPUT_NAME].join(", ")}) in ${elapsed}ms`);
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
