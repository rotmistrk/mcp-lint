use syn::visit::Visit;
use syn::spanned::Spanned;
use syn::{self, Expr, ExprField, ExprMethodCall, ExprMacro, ImplItemFn, ItemFn, ItemStruct, Stmt, Visibility};

use crate::types::{Config, Violation};

/// Check a Rust source file against coding standards.
pub fn check(source: &str, path: &str, cfg: &Config) -> Vec<Violation> {
    let mut violations = check_line_width(source, cfg);
    violations.extend(check_code_lines(source, cfg));

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
        struct_count: 0,
    };
    visitor.visit_file(&file);
    if visitor.cfg.rust.max_structs_per_file > 0
        && visitor.struct_count > visitor.cfg.rust.max_structs_per_file
    {
        visitor.violations.push(Violation {
            line: 1,
            rule: "max-structs-per-file".into(),
            message: format!(
                "file has {} structs (max {}); move each struct to its own file",
                visitor.struct_count, visitor.cfg.rust.max_structs_per_file
            ),
            severity: "error".into(),
        });
    }
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

fn check_code_lines(source: &str, cfg: &Config) -> Vec<Violation> {
    if cfg.max_code_lines_per_file == 0 {
        return Vec::new();
    }
    let count = source
        .lines()
        .filter(|line| {
            let trimmed = line.trim();
            !trimmed.is_empty() && !trimmed.starts_with("//")
        })
        .count();
    if count > cfg.max_code_lines_per_file {
        vec![Violation {
            line: 1,
            rule: "file-length".into(),
            message: format!(
                "file has {} code lines (max {}); split into separate files by responsibility. Do NOT remove comments or blank lines to reduce count",
                count, cfg.max_code_lines_per_file
            ),
            severity: "error".into(),
        }]
    } else {
        Vec::new()
    }
}

/// Returns true if the expression is or contains a method call in its access chain.
fn has_method_call(expr: &Expr) -> bool {
    match expr {
        Expr::MethodCall(_) => true,
        Expr::Field(f) => has_method_call(&f.base),
        Expr::Paren(p) => has_method_call(&p.expr),
        _ => false,
    }
}

/// Returns true if the immediate receiver of this call is a `?` (try) expression.
fn contains_try(expr: &Expr) -> bool {
    match expr {
        Expr::Try(_) => true,
        Expr::Paren(p) => contains_try(&p.expr),
        _ => false,
    }
}

/// Counts the depth of consecutive named field accesses (not through method calls).
fn field_chain_depth(expr: &Expr) -> usize {
    match expr {
        Expr::Field(f) => {
            if let syn::Member::Named(_) = &f.member {
                1 + field_chain_depth(&f.base)
            } else {
                0
            }
        }
        _ => 0,
    }
}

/// Total depth of a field expression chain (including itself).
fn total_field_depth(node: &ExprField) -> usize {
    if let syn::Member::Named(_) = &node.member {
        1 + field_chain_depth(&node.base)
    } else {
        0
    }
}

struct RustVisitor<'a> {
    cfg: &'a Config,
    is_test: bool,
    violations: Vec<Violation>,
    struct_count: usize,
}

impl<'a> RustVisitor<'a> {
    fn check_fn(
        &mut self,
        name: &str,
        start_line: usize,
        params_count: usize,
        block: &syn::Block,
        is_constructor: bool,
    ) {
        let end_line = block_end_line(block, start_line);
        let length = end_line.saturating_sub(start_line).saturating_sub(1);

        if length > self.cfg.max_method_length {
            self.violations.push(Violation {
                line: start_line,
                rule: "method-length".into(),
                message: format!(
                    "{name} is {length} lines (max {}); extract conceptually distinct steps into helper functions. Do NOT remove comments, collapse multi-line expressions, or remove blank lines",
                    self.cfg.max_method_length
                ),
                severity: "error".into(),
            });
        }

        if params_count > self.cfg.max_params {
            let (severity, hint) = if is_constructor {
                ("warning", "; consider splitting the struct into smaller cohesive types, or use a builder/config struct")
            } else {
                ("error", "; group related parameters into a struct, or extract common args into a separate trait")
            };
            self.violations.push(Violation {
                line: start_line,
                rule: "param-count".into(),
                message: format!(
                    "{name} has {params_count} parameters (max {}){hint}",
                    self.cfg.max_params
                ),
                severity: severity.into(),
            });
        }

        let depth = max_nesting_depth(block);
        if depth > self.cfg.max_nesting_depth {
            self.violations.push(Violation {
                line: start_line,
                rule: "nesting-depth".into(),
                message: format!(
                    "{name} has nesting depth {depth} (max {}); extract loop bodies or branch bodies into named helper functions",
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
        self.check_fn(&name, start, params, &node.block, false);
        syn::visit::visit_item_fn(self, node);
    }

    fn visit_impl_item_fn(&mut self, node: &'a ImplItemFn) {
        let name = node.sig.ident.to_string();
        let start = node.sig.ident.span().start().line;
        let params = node.sig.inputs.len();
        self.check_fn(&name, start, params, &node.block, name == "new");
        syn::visit::visit_impl_item_fn(self, node);
    }

    fn visit_expr_method_call(&mut self, node: &'a ExprMethodCall) {
        if self.cfg.rust.forbid_mid_chain_try {
            if contains_try(&node.receiver) {
                let line = node.method.span().start().line;
                self.violations.push(Violation {
                    line,
                    rule: "no-mid-chain-try".into(),
                    message: "? in middle of chain hides error origin; handle the error before continuing the chain".into(),
                    severity: "error".into(),
                });
            }
        }
        // Check unwrap/expect (existing logic is in UnwrapFinder)
        syn::visit::visit_expr_method_call(self, node);
    }

    fn visit_item_struct(&mut self, node: &'a ItemStruct) {
        self.struct_count += 1;
        if self.cfg.rust.forbid_pub_fields && !matches!(node.vis, Visibility::Inherited) {
            let struct_name = node.ident.to_string();
            for field in &node.fields {
                if matches!(field.vis, Visibility::Public(_)) {
                    let line = field
                        .ident
                        .as_ref()
                        .map(|i| i.span().start().line)
                        .unwrap_or(node.ident.span().start().line);
                    let field_name = field
                        .ident
                        .as_ref()
                        .map(|i| i.to_string())
                        .unwrap_or_else(|| "unnamed".into());
                    self.violations.push(Violation {
                        line,
                        rule: "no-pub-fields".into(),
                        message: format!(
                            "{struct_name}.{field_name}: public fields break encapsulation; use methods"
                        ),
                        severity: "error".into(),
                    });
                }
            }
        }
        syn::visit::visit_item_struct(self, node);
    }

    fn visit_expr_field(&mut self, node: &'a ExprField) {
        if self.cfg.rust.forbid_field_on_method {
            if let syn::Member::Named(ident) = &node.member {
                if has_method_call(&node.base) {
                    let line = ident.span().start().line;
                    let field_name = ident.to_string();
                    self.violations.push(Violation {
                        line,
                        rule: "no-field-on-method".into(),
                        message: format!(
                            ".{field_name}: field access after method call breaks encapsulation; use an accessor method instead"
                        ),
                        severity: "error".into(),
                    });
                } else {
                    // Check deep field chains — only at the outermost node
                    let depth = total_field_depth(node);
                    if depth >= 3 {
                        let line = ident.span().start().line;
                        self.violations.push(Violation {
                            line,
                            rule: "no-deep-field-access".into(),
                            message: format!(
                                "field chain depth {} breaks encapsulation; add a method to an intermediate type",
                                depth
                            ),
                            severity: "error".into(),
                        });
                        // Don't recurse into field children — avoid duplicate reports
                        return;
                    }
                }
            }
        }
        syn::visit::visit_expr_field(self, node);
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

    fn visit_expr_macro(&mut self, node: &'a ExprMacro) {
        self.check_panic_macro(&node.mac);
        syn::visit::visit_expr_macro(self, node);
    }

    fn visit_stmt_macro(&mut self, node: &'a syn::StmtMacro) {
        self.check_panic_macro(&node.mac);
        syn::visit::visit_stmt_macro(self, node);
    }

    fn visit_path(&mut self, node: &'a syn::Path) {
        if self.cfg.rust.forbid_deep_path && node.segments.len() > 2 {
            let line = node.span().start().line;
            let path_str: String = node
                .segments
                .iter()
                .map(|s| s.ident.to_string())
                .collect::<Vec<_>>()
                .join("::");
            self.violations.push(Violation {
                line,
                rule: "no-deep-path".into(),
                message: format!(
                    "{path_str}: use a `use` import instead of fully qualified path"
                ),
                severity: "error".into(),
            });
        }
        syn::visit::visit_path(self, node);
    }
}

impl<'a> UnwrapFinder<'a> {
    fn check_panic_macro(&mut self, mac: &syn::Macro) {
        if !self.cfg.rust.forbid_panic {
            return;
        }
        let name = mac.path.segments.last().map(|s| s.ident.to_string());
        if let Some(name) = name {
            if matches!(name.as_str(), "panic" | "todo" | "unimplemented") {
                let line = mac.path.span().start().line;
                self.violations.push(Violation {
                    line,
                    rule: "no-panic".into(),
                    message: format!(
                        "{}!() is forbidden in non-test code; handle errors gracefully",
                        name
                    ),
                    severity: "error".into(),
                });
            }
        }
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
    fn test_panic_macro() {
        let src = "fn bad() {\n    panic!(\"boom\");\n}\n";
        let v = check(src, "lib.rs", &cfg());
        assert!(
            v.iter().any(|v| v.rule == "no-panic"),
            "expected no-panic: {v:?}"
        );
    }

    #[test]
    fn test_todo_macro() {
        let src = "fn bad() {\n    todo!();\n}\n";
        let v = check(src, "lib.rs", &cfg());
        assert!(
            v.iter().any(|v| v.rule == "no-panic"),
            "expected no-panic for todo!: {v:?}"
        );
    }

    #[test]
    fn test_panic_allowed_in_tests() {
        let src = "#[cfg(test)]\nfn test_ok() {\n    panic!(\"ok\");\n}\n";
        let v = check(src, "lib.rs", &cfg());
        assert!(
            !v.iter().any(|v| v.rule == "no-panic"),
            "panic should be allowed in test code"
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

    #[test]
    fn test_deep_path() {
        let src = "fn bad() {\n    let _ = std::collections::HashMap::new();\n}\n";
        let v = check(src, "lib.rs", &cfg());
        assert!(
            v.iter().any(|v| v.rule == "no-deep-path"),
            "expected no-deep-path: {v:?}"
        );
    }

    #[test]
    fn test_shallow_path_ok() {
        let src = "fn ok() {\n    let _ = Vec::new();\n}\n";
        let v = check(src, "lib.rs", &cfg());
        assert!(
            !v.iter().any(|v| v.rule == "no-deep-path"),
            "single :: should be allowed: {v:?}"
        );
    }
}
