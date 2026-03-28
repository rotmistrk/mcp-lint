import ts from "typescript";
import { readFileSync } from "fs";
import type { Config, Violation } from "./types.js";

export function check(path: string, cfg: Config): Violation[] {
  const source = readFileSync(path, "utf-8");
  const violations: Violation[] = [];

  violations.push(...checkLineWidth(source, cfg));

  const sourceFile = ts.createSourceFile(
    path,
    source,
    ts.ScriptTarget.Latest,
    true,
  );

  const isTest =
    path.endsWith(".test.ts") ||
    path.endsWith(".test.tsx") ||
    path.endsWith(".spec.ts") ||
    path.endsWith(".spec.tsx");

  visitNode(sourceFile, sourceFile, cfg, isTest, violations);
  return violations;
}

function checkLineWidth(source: string, cfg: Config): Violation[] {
  const violations: Violation[] = [];
  const lines = source.split("\n");
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].length > cfg.max_line_width) {
      violations.push({
        line: i + 1,
        rule: "line-width",
        message: `line is ${lines[i].length} chars (max ${cfg.max_line_width})`,
        severity: "error",
      });
    }
  }
  return violations;
}

function visitNode(
  node: ts.Node,
  sourceFile: ts.SourceFile,
  cfg: Config,
  isTest: boolean,
  violations: Violation[],
): void {
  // Check functions/methods for length, nesting, params
  if (
    ts.isFunctionDeclaration(node) ||
    ts.isMethodDeclaration(node) ||
    ts.isArrowFunction(node) ||
    ts.isFunctionExpression(node)
  ) {
    checkFunction(node, sourceFile, cfg, violations);
  }

  // Forbid `any` type
  if (cfg.typescript.forbid_any && ts.isTypeReferenceNode(node)) {
    const name = node.typeName.getText(sourceFile);
    if (name === "any") {
      violations.push({
        line: getLine(node, sourceFile),
        rule: "no-any",
        message: "use 'unknown' or a specific type instead of 'any'",
        severity: "error",
      });
    }
  }

  // Catch `: any` and `as any` via keyword token
  if (node.kind === ts.SyntaxKind.AnyKeyword) {
    violations.push({
      line: getLine(node, sourceFile),
      rule: "no-any",
      message: "use 'unknown' or a specific type instead of 'any'",
      severity: "error",
    });
  }

  // Forbid class components
  if (cfg.typescript.forbid_class_components && ts.isClassDeclaration(node)) {
    checkClassComponent(node, sourceFile, violations);
  }

  // Forbid waitForTimeout in tests
  if (isTest && cfg.typescript.forbid_wait_for_timeout) {
    checkWaitForTimeout(node, sourceFile, violations);
  }

  ts.forEachChild(node, (child) =>
    visitNode(child, sourceFile, cfg, isTest, violations),
  );
}

function checkFunction(
  node: ts.Node,
  sourceFile: ts.SourceFile,
  cfg: Config,
  violations: Violation[],
): void {
  const startLine = getLine(node, sourceFile);
  const endLine = getEndLine(node, sourceFile);
  const length = endLine - startLine - 1;
  const name = getFunctionName(node, sourceFile);

  if (length > cfg.max_method_length) {
    violations.push({
      line: startLine,
      rule: "method-length",
      message: `${name} is ${length} lines (max ${cfg.max_method_length})`,
      severity: "error",
    });
  }

  // Parameter count
  if ("parameters" in node) {
    const params = (node as ts.FunctionLikeDeclaration).parameters;
    if (params.length > cfg.max_params) {
      violations.push({
        line: startLine,
        rule: "param-count",
        message: `${name} has ${params.length} parameters (max ${cfg.max_params})`,
        severity: "error",
      });
    }
  }
}

function checkClassComponent(
  node: ts.ClassDeclaration,
  sourceFile: ts.SourceFile,
  violations: Violation[],
): void {
  if (!node.heritageClauses) return;
  for (const clause of node.heritageClauses) {
    if (clause.token !== ts.SyntaxKind.ExtendsKeyword) continue;
    for (const type of clause.types) {
      const text = type.expression.getText(sourceFile);
      if (
        text === "Component" ||
        text === "PureComponent" ||
        text === "React.Component" ||
        text === "React.PureComponent"
      ) {
        violations.push({
          line: getLine(node, sourceFile),
          rule: "no-class-component",
          message: "use functional components with hooks",
          severity: "error",
        });
      }
    }
  }
}

function checkWaitForTimeout(
  node: ts.Node,
  sourceFile: ts.SourceFile,
  violations: Violation[],
): void {
  if (!ts.isCallExpression(node)) return;
  const expr = node.expression;
  if (!ts.isPropertyAccessExpression(expr)) return;
  if (expr.name.text === "waitForTimeout") {
    violations.push({
      line: getLine(node, sourceFile),
      rule: "no-wait-for-timeout",
      message: "use explicit wait conditions instead of waitForTimeout",
      severity: "error",
    });
  }
}

function getLine(node: ts.Node, sourceFile: ts.SourceFile): number {
  return sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile))
    .line + 1;
}

function getEndLine(node: ts.Node, sourceFile: ts.SourceFile): number {
  return sourceFile.getLineAndCharacterOfPosition(node.getEnd()).line + 1;
}

function getFunctionName(node: ts.Node, sourceFile: ts.SourceFile): string {
  if (ts.isFunctionDeclaration(node) || ts.isMethodDeclaration(node)) {
    return node.name?.getText(sourceFile) ?? "(anonymous)";
  }
  // Arrow function — try to get variable name
  if (ts.isVariableDeclaration(node.parent)) {
    return node.parent.name.getText(sourceFile);
  }
  return "(anonymous)";
}
