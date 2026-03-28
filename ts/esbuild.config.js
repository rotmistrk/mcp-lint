import { build } from "esbuild";

await build({
  entryPoints: ["dist/main.js"],
  bundle: true,
  platform: "node",
  target: "node20",
  format: "cjs",
  banner: { js: "#!/usr/bin/env node" },
  outfile: "dist/mcp-lint-ts.cjs",
  external: [],
});
