package dev.rotmistr.mcplint;

import com.github.javaparser.ast.CompilationUnit;
import com.github.javaparser.ast.body.ClassOrInterfaceDeclaration;
import com.github.javaparser.ast.body.ConstructorDeclaration;
import com.github.javaparser.ast.body.MethodDeclaration;
import com.github.javaparser.ast.body.Parameter;
import com.github.javaparser.ast.type.ClassOrInterfaceType;

import java.util.HashSet;
import java.util.List;
import java.util.Set;

final class ConcreteDepsChecker {

    static void check(CompilationUnit cu, List<Violation> violations) {
        var interfaces = new HashSet<String>();
        var classes = new HashSet<String>();
        cu.findAll(ClassOrInterfaceDeclaration.class).forEach(decl -> {
            if (decl.isInterface()) {
                interfaces.add(decl.getNameAsString());
            } else {
                classes.add(decl.getNameAsString());
            }
        });

        cu.findAll(MethodDeclaration.class).forEach(m -> {
            for (var param : m.getParameters()) {
                checkParam(param, m.getNameAsString(), interfaces, classes, violations);
            }
        });
        cu.findAll(ConstructorDeclaration.class).forEach(c -> {
            for (var param : c.getParameters()) {
                checkParam(param, c.getNameAsString(), interfaces, classes, violations);
            }
        });
    }

    private static void checkParam(
            Parameter param,
            String methodName,
            Set<String> interfaces,
            Set<String> classes,
            List<Violation> violations
    ) {
        var type = param.getType();
        if (!(type instanceof ClassOrInterfaceType cit)) return;
        var typeName = cit.getNameAsString();
        if (isPrimitiveWrapper(typeName)) return;
        if (isKnownInterface(typeName)) return;
        if (interfaces.contains(typeName)) return;
        if (!classes.contains(typeName)) return;
        int line = param.getBegin().map(p -> p.line).orElse(0);
        violations.add(new Violation(
                line, "no-concrete-deps",
                methodName + ": parameter " + param.getNameAsString() +
                        " has concrete type " + typeName + "; use an interface",
                "error"));
    }

    private static boolean isPrimitiveWrapper(String name) {
        return switch (name) {
            case "String", "Integer", "Long", "Double", "Float",
                 "Boolean", "Byte", "Short", "Character", "Void",
                 "BigDecimal", "BigInteger", "Object",
                 "LocalDate", "LocalDateTime", "Instant", "Duration",
                 "UUID", "URI", "URL", "Path", "File" -> true;
            default -> false;
        };
    }

    private static boolean isKnownInterface(String name) {
        return switch (name) {
            case "List", "Map", "Set", "Collection", "Iterable",
                 "Iterator", "Stream", "Optional",
                 "Comparable", "Comparator", "Runnable", "Callable",
                 "Function", "Supplier", "Consumer", "Predicate",
                 "Future", "CompletionStage",
                 "Serializable", "Cloneable", "AutoCloseable", "Closeable",
                 "CharSequence", "Appendable", "Readable" -> true;
            default -> false;
        };
    }
}
