import { readFileSync } from "fs";
import { check } from "./checker.js";
import { defaults } from "./types.js";
import type { Config } from "./types.js";

const args = process.argv.slice(2);

if (args.length < 2 || args[0] !== "check") {
  process.stderr.write("Usage: mcp-lint-ts check <file.ts> [--config-json <json>]\n");
  process.exit(2);
}

const path = args[1];
const cfg = parseConfig(args);
const violations = check(path, cfg);

process.stdout.write(JSON.stringify(violations, null, 2) + "\n");

if (violations.length > 0) {
  process.exit(1);
}

function parseConfig(args: string[]): Config {
  const idx = args.indexOf("--config-json");
  if (idx >= 0 && args[idx + 1]) {
    try {
      return { ...defaults, ...JSON.parse(args[idx + 1]) };
    } catch {
      // fall through to defaults
    }
  }
  return defaults;
}
