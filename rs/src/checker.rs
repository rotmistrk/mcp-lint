use syn::visit::Visit;
use syn::{self, Expr, ExprMethodCall, ImplItemFn, ItemFn, Stmt};

use crate::types::{Config, Violation};

/// Check a Rust source file against coding standards.
pub fn check(source: &str, path: &str, cfg: &Config) -> Vec<Violation> {
    let mut violations = check_line_width(source, cfg);

    let is_test = path.ends_with("_test.rs")
        || path.contains("/tests/")
        || source.contains("#[cfg(test)]");

    let file = match syn::parse_file(source) {
        Ok(f) => f,
        Err(e) => {
            violations.push(Violation {
                line: e.span().start().line,
                rule: "parse-error".into(),
                message: format!("failed to parse: {e}"),
                severity: "error".into(),
            });
            return violations;
        }
    };

    let mut visitor = RustVisitor {
        cfg,
        is_test,
        violations: Vec::new(),
    };
    visitor.visit_file(&file);
    violations.extend(visitor.violations);
    violations
}

fn check_line_width(source: &str, cfg: &Config) -> Vec<Violation> {
    source
        .lines()
        .enumerate()
        .filter(|(_, line)| line.len() > cfg.max_line_width)
        .map(|(i, line)| Violation {
            line: i + 1,
            rule: "line-width".into(),
            message: format!(
                "line is {} chars (max {})",
                line.len(),
                cfg.max_line_width
            ),
            severity: "error".into(),
        })
        .collect()
}

struct RustVisitor<'a> {
    cfg: &'a Config,
    is_test: bool,
    violations: Vec<Violation>,
}

impl<'a> RustVisitor<'a> {
    fn check_fn(
        &mut self,
        name: &str,
        start_line: usize,
        params_count: usize,
        block: &syn::Block,
    ) {
        let end_line = block_end_line(block, start_line);
        let length = end_line.saturating_sub(start_line).saturating_sub(1);

        if length > self.cfg.max_method_length {
            self.violations.push(Violation {
                line: start_line,
                rule: "method-length".into(),
                message: format!(
                    "{name} is {length} lines (max {})",
                    self.cfg.max_method_length
                ),
                severity: "error".into(),
            });
        }

        if params_count > self.cfg.max_params {
            self.violations.push(Violation {
                line: start_line,
                rule: "param-count".into(),
                message: format!(
                    "{name} has {params_count} parameters (max {})",
                    self.cfg.max_params
                ),
                severity: "error".into(),
            });
        }

        let depth = max_nesting_depth(block);
        if depth > self.cfg.max_nesting_depth {
            self.violations.push(Violation {
                line: start_line,
                rule: "nesting-depth".into(),
                message: format!(
                    "{name} has nesting depth {depth} (max {})",
                    self.cfg.max_nesting_depth
                ),
                severity: "error".into(),
            });
        }

        if !self.is_test {
            self.check_unwrap_expect(block);
        }
    }

    fn check_unwrap_expect(&mut self, block: &syn::Block) {
        let mut finder = UnwrapFinder {
            cfg: self.cfg,
            violations: Vec::new(),
        };
        syn::visit::visit_block(&mut finder, block);
        self.violations.extend(finder.violations);
    }
}

impl<'a> Visit<'a> for RustVisitor<'a> {
    fn visit_item_fn(&mut self, node: &'a ItemFn) {
        let name = node.sig.ident.to_string();
        let start = node.sig.ident.span().start().line;
        let params = node.sig.inputs.len();
        self.check_fn(&name, start, params, &node.block);
        syn::visit::visit_item_fn(self, node);
    }

    fn visit_impl_item_fn(&mut self, node: &'a ImplItemFn) {
        let name = node.sig.ident.to_string();
        let start = node.sig.ident.span().start().line;
        let params = node.sig.inputs.len();
        self.check_fn(&name, start, params, &node.block);
        syn::visit::visit_impl_item_fn(self, node);
    }
}

struct UnwrapFinder<'a> {
    cfg: &'a Config,
    violations: Vec<Violation>,
}

impl<'a> Visit<'a> for UnwrapFinder<'a> {
    fn visit_expr_method_call(&mut self, node: &'a ExprMethodCall) {
        let method = node.method.to_string();
        let line = node.method.span().start().line;

        if self.cfg.rust.forbid_unwrap && method == "unwrap" {
            self.violations.push(Violation {
                line,
                rule: "no-unwrap".into(),
                message: "unwrap() is forbidden in non-test code; use ? or match"
                    .into(),
                severity: "error".into(),
            });
        }
        if self.cfg.rust.forbid_expect && method == "expect" {
            self.violations.push(Violation {
                line,
                rule: "no-expect".into(),
                message: "expect() is forbidden in non-test code; use ? or match"
                    .into(),
                severity: "error".into(),
            });
        }
        syn::visit::visit_expr_method_call(self, node);
    }
}

fn block_end_line(block: &syn::Block, _fallback: usize) -> usize {
    block.brace_token.span.close().end().line
}

fn max_nesting_depth(block: &syn::Block) -> usize {
    let mut max = 0;
    for stmt in &block.stmts {
        let d = stmt_nesting(stmt, 0);
        if d > max {
            max = d;
        }
    }
    max
}

fn stmt_nesting(stmt: &Stmt, depth: usize) -> usize {
    match stmt {
        Stmt::Expr(e, _) => expr_nesting(e, depth),
        _ => depth,
    }
}

fn expr_nesting(expr: &Expr, depth: usize) -> usize {
    match expr {
        Expr::If(e) => {
            let d = depth + 1;
            let mut max = block_nesting(&e.then_branch, d);
            if let Some((_, else_branch)) = &e.else_branch {
                let ed = expr_nesting(else_branch, depth);
                if ed > max {
                    max = ed;
                }
            }
            max
        }
        Expr::ForLoop(e) => block_nesting(&e.body, depth + 1),
        Expr::While(e) => block_nesting(&e.body, depth + 1),
        Expr::Loop(e) => block_nesting(&e.body, depth + 1),
        Expr::Match(e) => {
            let d = depth + 1;
            let mut max = d;
            for arm in &e.arms {
                let ad = expr_nesting(&arm.body, d);
                if ad > max {
                    max = ad;
                }
            }
            max
        }
        Expr::Block(e) => block_nesting(&e.block, depth),
        _ => depth,
    }
}

fn block_nesting(block: &syn::Block, depth: usize) -> usize {
    let mut max = depth;
    for stmt in &block.stmts {
        let d = stmt_nesting(stmt, depth);
        if d > max {
            max = d;
        }
    }
    max
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::Config;

    fn cfg() -> Config {
        Config::default()
    }

    #[test]
    fn test_clean() {
        let v = check("fn short() { let _ = 1; }\n", "lib.rs", &cfg());
        assert!(v.is_empty(), "expected no violations: {v:?}");
    }

    #[test]
    fn test_unwrap() {
        let src = "fn bad() {\n    let x: Option<i32> = Some(1);\n    let _ = x.unwrap();\n}\n";
        let v = check(src, "lib.rs", &cfg());
        assert!(
            v.iter().any(|v| v.rule == "no-unwrap"),
            "expected no-unwrap: {v:?}"
        );
    }

    #[test]
    fn test_expect() {
        let src =
            "fn bad() {\n    let x: Option<i32> = Some(1);\n    let _ = x.expect(\"boom\");\n}\n";
        let v = check(src, "lib.rs", &cfg());
        assert!(
            v.iter().any(|v| v.rule == "no-expect"),
            "expected no-expect: {v:?}"
        );
    }

    #[test]
    fn test_unwrap_allowed_in_tests() {
        let src = "#[cfg(test)]\nfn test_ok() {\n    let _ = Some(1).unwrap();\n}\n";
        let v = check(src, "lib.rs", &cfg());
        assert!(
            !v.iter().any(|v| v.rule == "no-unwrap"),
            "unwrap should be allowed in test code"
        );
    }

    #[test]
    fn test_nesting_depth() {
        let src = "fn deep() {\n    if true {\n        if true {\n            if true {\n                if true {\n                    let _ = 1;\n                }\n            }\n        }\n    }\n}\n";
        let v = check(src, "lib.rs", &cfg());
        assert!(
            v.iter().any(|v| v.rule == "nesting-depth"),
            "expected nesting-depth: {v:?}"
        );
    }

    #[test]
    fn test_param_count() {
        let src = "fn many(a: i32, b: i32, c: i32, d: i32, e: i32, f: i32, g: i32, h: i32) {\n    let _ = a;\n}\n";
        let v = check(src, "lib.rs", &cfg());
        assert!(
            v.iter().any(|v| v.rule == "param-count"),
            "expected param-count: {v:?}"
        );
    }

    #[test]
    fn test_line_width() {
        let long = format!("fn x() {{ let _ = \"{}\"; }}\n", "a".repeat(120));
        let v = check(&long, "lib.rs", &cfg());
        assert!(
            v.iter().any(|v| v.rule == "line-width"),
            "expected line-width: {v:?}"
        );
    }
}
