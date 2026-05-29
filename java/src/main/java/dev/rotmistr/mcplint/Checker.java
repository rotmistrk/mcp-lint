package dev.rotmistr.mcplint;

import com.github.javaparser.StaticJavaParser;
import com.github.javaparser.ast.CompilationUnit;
import com.github.javaparser.ast.Modifier;
import com.github.javaparser.ast.body.CallableDeclaration;
import com.github.javaparser.ast.body.ClassOrInterfaceDeclaration;
import com.github.javaparser.ast.body.ConstructorDeclaration;
import com.github.javaparser.ast.body.FieldDeclaration;
import com.github.javaparser.ast.body.MethodDeclaration;
import com.github.javaparser.ast.expr.FieldAccessExpr;
import com.github.javaparser.ast.expr.NameExpr;
import com.github.javaparser.ast.stmt.*;
import com.github.javaparser.ast.type.ClassOrInterfaceType;

import java.util.ArrayList;
import java.util.List;

public final class Checker {

    public static List<Violation> check(String source, String path, Config cfg) {
        var violations = new ArrayList<Violation>();
        checkLineWidth(source, cfg, violations);
        checkCodeLines(source, cfg, violations);

        CompilationUnit cu;
        try {
            cu = StaticJavaParser.parse(source);
        } catch (Exception e) {
            violations.add(new Violation(1, "parse-error", "failed to parse: " + e.getMessage(), "error"));
            return violations;
        }

        checkMethods(cu, cfg, violations);
        checkConstructors(cu, cfg, violations);
        if (cfg.java.forbid_raw_types) {
            checkRawTypes(cu, violations);
        }
        if (cfg.java.forbid_public_fields) {
            checkPublicFields(cu, violations);
        }
        checkClassCount(cu, cfg, violations);
        if (cfg.java.forbid_deep_access) {
            checkDeepAccess(cu, violations);
        }
        if (cfg.java.forbid_concrete_deps) {
            ConcreteDepsChecker.check(cu, violations);
        }
        return violations;
    }

    private static void checkLineWidth(String source, Config cfg, List<Violation> violations) {
        var lines = source.split("\n", -1);
        for (int i = 0; i < lines.length; i++) {
            if (lines[i].length() > cfg.max_line_width) {
                violations.add(new Violation(
                        i + 1, "line-width",
                        "line is " + lines[i].length() + " chars (max " + cfg.max_line_width + ")",
                        "error"));
            }
        }
    }

    private static void checkCodeLines(String source, Config cfg, List<Violation> violations) {
        if (cfg.max_code_lines_per_file <= 0) return;
        var lines = source.split("\n", -1);
        int count = 0;
        for (var line : lines) {
            var trimmed = line.trim();
            if (!trimmed.isEmpty() && !trimmed.startsWith("//")) {
                count++;
            }
        }
        if (count > cfg.max_code_lines_per_file) {
            violations.add(new Violation(
                    1, "file-length",
                    "file has " + count + " code lines (max " + cfg.max_code_lines_per_file + ")",
                    "error"));
        }
    }

    private static void checkMethods(CompilationUnit cu, Config cfg, List<Violation> violations) {
        cu.findAll(MethodDeclaration.class).forEach(m -> {
            if (m.getBody().isEmpty()) return;
            checkCallable(m, m.getNameAsString(), cfg, violations);
        });
    }

    private static void checkConstructors(CompilationUnit cu, Config cfg, List<Violation> violations) {
        cu.findAll(ConstructorDeclaration.class).forEach(c ->
                checkCallable(c, c.getNameAsString(), cfg, violations));
    }

    private static void checkCallable(
            CallableDeclaration<?> decl,
            String name,
            Config cfg,
            List<Violation> violations
    ) {
        int startLine = decl.getBegin().map(p -> p.line).orElse(0);
        int endLine = decl.getEnd().map(p -> p.line).orElse(0);
        int length = endLine - startLine - 1;

        if (length > cfg.max_method_length) {
            violations.add(new Violation(
                    startLine, "method-length",
                    name + " is " + length + " lines (max " + cfg.max_method_length + ")",
                    "error"));
        }

        int params = decl.getParameters().size();
        if (params > cfg.max_params) {
            violations.add(new Violation(
                    startLine, "param-count",
                    name + " has " + params + " parameters (max " + cfg.max_params + ")",
                    "error"));
        }

        int depth = maxNesting(decl, 0);
        if (depth > cfg.max_nesting_depth) {
            violations.add(new Violation(
                    startLine, "nesting-depth",
                    name + " has nesting depth " + depth + " (max " + cfg.max_nesting_depth + ")",
                    "error"));
        }
    }

    private static int maxNesting(com.github.javaparser.ast.Node node, int depth) {
        int max = depth;
        for (var child : node.getChildNodes()) {
            int childDepth = depth;
            if (child instanceof IfStmt
                    || child instanceof ForStmt
                    || child instanceof ForEachStmt
                    || child instanceof WhileStmt
                    || child instanceof DoStmt
                    || child instanceof SwitchStmt) {
                childDepth = depth + 1;
            }
            int sub = maxNesting(child, childDepth);
            if (sub > max) max = sub;
        }
        return max;
    }

    private static void checkRawTypes(CompilationUnit cu, List<Violation> violations) {
        cu.findAll(ClassOrInterfaceType.class).forEach(type -> {
            if (type.getTypeArguments().isEmpty() && isGenericType(type.getNameAsString())) {
                int line = type.getBegin().map(p -> p.line).orElse(0);
                violations.add(new Violation(
                        line, "no-raw-types",
                        "raw type " + type.getNameAsString() + "; use parameterized type",
                        "warning"));
            }
        });
    }

    private static void checkPublicFields(CompilationUnit cu, List<Violation> violations) {
        cu.findAll(FieldDeclaration.class).forEach(field -> {
            if (!field.hasModifier(Modifier.Keyword.PUBLIC)) return;
            if (field.hasModifier(Modifier.Keyword.STATIC) && field.hasModifier(Modifier.Keyword.FINAL)) return;
            var parent = field.getParentNode().orElse(null);
            String className = "(unknown)";
            if (parent instanceof ClassOrInterfaceDeclaration cid) {
                className = cid.getNameAsString();
                if (cid.hasModifier(Modifier.Keyword.PRIVATE)) return;
            }
            for (var variable : field.getVariables()) {
                int line = variable.getBegin().map(p -> p.line).orElse(0);
                violations.add(new Violation(
                        line, "no-public-fields",
                        className + "." + variable.getNameAsString() +
                                ": public fields break encapsulation; use methods",
                        "error"));
            }
        });
    }

    private static void checkClassCount(CompilationUnit cu, Config cfg, List<Violation> violations) {
        if (cfg.java.max_classes_per_file <= 0) return;
        var topLevel = cu.findAll(ClassOrInterfaceDeclaration.class).stream()
                .filter(c -> c.getParentNode().orElse(null) instanceof CompilationUnit)
                .count();
        if (topLevel > cfg.java.max_classes_per_file) {
            violations.add(new Violation(
                    1, "max-classes-per-file",
                    "file has " + topLevel + " top-level classes (max " + cfg.java.max_classes_per_file + ")",
                    "error"));
        }
    }

    private static void checkDeepAccess(CompilationUnit cu, List<Violation> violations) {
        cu.findAll(FieldAccessExpr.class).forEach(fa -> {
            // Count depth: a.b = 1, a.b.c = 2
            int depth = 0;
            var scope = fa.getScope();
            while (scope instanceof FieldAccessExpr inner) {
                depth++;
                scope = inner.getScope();
            }
            if (depth >= 2 && scope instanceof NameExpr) {
                int line = fa.getBegin().map(p -> p.line).orElse(0);
                violations.add(new Violation(
                        line, "no-deep-access",
                        fa.toString() + ": use an import instead of fully qualified name",
                        "error"));
            }
        });
    }

    private static boolean isGenericType(String name) {
        return switch (name) {
            case "List", "Map", "Set", "Collection", "Iterator",
                 "Optional", "Stream", "Iterable",
                 "ArrayList", "HashMap", "HashSet", "LinkedList",
                 "TreeMap", "TreeSet", "LinkedHashMap", "LinkedHashSet",
                 "Queue", "Deque", "ArrayDeque", "PriorityQueue",
                 "Comparable", "Comparator", "Future", "CompletableFuture" -> true;
            default -> false;
        };
    }
}
