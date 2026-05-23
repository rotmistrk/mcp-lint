import ts from "typescript";
import { readFileSync } from "fs";
import type { Config, Violation } from "./types.js";

export function check(path: string, cfg: Config): Violation[] {
  const source = readFileSync(path, "utf-8");
  const violations: Violation[] = [];

  violations.push(...checkLineWidth(source, cfg));
  violations.push(...checkCodeLines(source, cfg));

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

  let classCount = 0;

  const visit = (node: ts.Node): void => {
    if (
      ts.isFunctionDeclaration(node) ||
      ts.isMethodDeclaration(node) ||
      ts.isArrowFunction(node) ||
      ts.isFunctionExpression(node)
    ) {
      checkFunction(node, sourceFile, cfg, violations);
    }

    if (cfg.typescript.forbid_any && node.kind === ts.SyntaxKind.AnyKeyword) {
      violations.push({
        line: getLine(node, sourceFile),
        rule: "no-any",
        message: "use 'unknown' or a specific type instead of 'any'",
        severity: "error",
      });
    }

    if (ts.isClassDeclaration(node)) {
      classCount++;
      if (cfg.typescript.forbid_class_components) {
        checkClassComponent(node, sourceFile, violations);
      }
      if (cfg.typescript.forbid_public_properties) {
        checkPublicProperties(node, sourceFile, violations);
      }
    }

    if (isTest && cfg.typescript.forbid_wait_for_timeout) {
      checkWaitForTimeout(node, sourceFile, violations);
    }

    ts.forEachChild(node, visit);
  };

  visit(sourceFile);

  if (
    cfg.typescript.max_classes_per_file > 0 &&
    classCount > cfg.typescript.max_classes_per_file
  ) {
    violations.push({
      line: 1,
      rule: "max-classes-per-file",
      message: `file has ${classCount} classes (max ${cfg.typescript.max_classes_per_file})`,
      severity: "error",
    });
  }

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

function checkCodeLines(source: string, cfg: Config): Violation[] {
  if (cfg.max_code_lines_per_file <= 0) return [];
  const count = source
    .split("\n")
    .filter((line) => {
      const trimmed = line.trim();
      return trimmed !== "" && !trimmed.startsWith("//");
    }).length;
  if (count > cfg.max_code_lines_per_file) {
    return [
      {
        line: 1,
        rule: "file-length",
        message: `file has ${count} code lines (max ${cfg.max_code_lines_per_file})`,
        severity: "error",
      },
    ];
  }
  return [];
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

function checkPublicProperties(
  node: ts.ClassDeclaration,
  sourceFile: ts.SourceFile,
  violations: Violation[],
): void {
  const className = node.name?.getText(sourceFile) ?? "(anonymous)";
  for (const member of node.members) {
    if (!ts.isPropertyDeclaration(member)) continue;
    const mods = ts.getModifiers(member);
    const hasPrivate = mods?.some(
      (m) =>
        m.kind === ts.SyntaxKind.PrivateKeyword ||
        m.kind === ts.SyntaxKind.ProtectedKeyword,
    );
    if (hasPrivate) continue;
    const name = member.name.getText(sourceFile);
    if (name.startsWith("#")) continue;
    violations.push({
      line: getLine(member, sourceFile),
      rule: "no-public-properties",
      message: `${className}.${name}: public properties break encapsulation; use private`,
      severity: "error",
    });
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
  return (
    sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1
  );
}

function getEndLine(node: ts.Node, sourceFile: ts.SourceFile): number {
  return sourceFile.getLineAndCharacterOfPosition(node.getEnd()).line + 1;
}

function getFunctionName(node: ts.Node, sourceFile: ts.SourceFile): string {
  if (ts.isFunctionDeclaration(node) || ts.isMethodDeclaration(node)) {
    return node.name?.getText(sourceFile) ?? "(anonymous)";
  }
  if (ts.isVariableDeclaration(node.parent)) {
    return node.parent.name.getText(sourceFile);
  }
  return "(anonymous)";
}
