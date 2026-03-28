import { describe, it } from "node:test";
import { strict as assert } from "node:assert";
import { writeFileSync, mkdtempSync, rmSync } from "fs";
import { join } from "path";
import { tmpdir } from "os";
import { check } from "./checker.js";
import { defaults } from "./types.js";

function tmpFile(name: string, content: string): string {
  const dir = mkdtempSync(join(tmpdir(), "mcp-lint-ts-"));
  const path = join(dir, name);
  writeFileSync(path, content);
  return path;
}

function hasRule(violations: { rule: string }[], rule: string): boolean {
  return violations.some((v) => v.rule === rule);
}

describe("TypeScript checker", () => {
  it("clean file has no violations", () => {
    const path = tmpFile("clean.ts", 'export const x: string = "hello";\n');
    const v = check(path, defaults);
    assert.equal(v.length, 0, `expected no violations: ${JSON.stringify(v)}`);
  });

  it("detects any type", () => {
    const path = tmpFile("bad.ts", "const x: any = 42;\n");
    const v = check(path, defaults);
    assert.ok(hasRule(v, "no-any"), `expected no-any: ${JSON.stringify(v)}`);
  });

  it("detects as any", () => {
    const path = tmpFile("cast.ts", "const x = foo as any;\n");
    const v = check(path, defaults);
    assert.ok(hasRule(v, "no-any"), `expected no-any: ${JSON.stringify(v)}`);
  });

  it("detects class component", () => {
    const path = tmpFile(
      "comp.tsx",
      "import React from 'react';\nclass Bad extends React.Component { render() { return null; } }\n",
    );
    const v = check(path, defaults);
    assert.ok(
      hasRule(v, "no-class-component"),
      `expected no-class-component: ${JSON.stringify(v)}`,
    );
  });

  it("detects waitForTimeout in test", () => {
    const path = tmpFile(
      "slow.test.ts",
      "await page.waitForTimeout(1000);\n",
    );
    const v = check(path, defaults);
    assert.ok(
      hasRule(v, "no-wait-for-timeout"),
      `expected no-wait-for-timeout: ${JSON.stringify(v)}`,
    );
  });

  it("allows waitForTimeout in non-test", () => {
    const path = tmpFile("util.ts", "await page.waitForTimeout(1000);\n");
    const v = check(path, defaults);
    assert.ok(
      !hasRule(v, "no-wait-for-timeout"),
      "waitForTimeout should only be flagged in test files",
    );
  });

  it("detects line width", () => {
    const path = tmpFile("wide.ts", `const x = "${"a".repeat(120)}";\n`);
    const v = check(path, defaults);
    assert.ok(
      hasRule(v, "line-width"),
      `expected line-width: ${JSON.stringify(v)}`,
    );
  });

  it("detects param count", () => {
    const path = tmpFile(
      "params.ts",
      "function many(a: number, b: number, c: number, d: number, e: number, f: number, g: number, h: number) { return a; }\n",
    );
    const v = check(path, defaults);
    assert.ok(
      hasRule(v, "param-count"),
      `expected param-count: ${JSON.stringify(v)}`,
    );
  });
});
