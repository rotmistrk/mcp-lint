package dev.rotmistr.mcplint;

public record Violation(int line, String rule, String message, String severity) {
}
