#include "checker.h"

#include <clang-c/Index.h>
#include <cstdlib>
#include <fstream>
#include <string>
#include <vector>

// --- Line width check (no AST needed) ---

static std::vector<Violation> check_line_width(
    const std::string& path,
    const Config& cfg
) {
    std::vector<Violation> violations;
    std::ifstream file(path);
    std::string line;
    int line_num = 0;

    while (std::getline(file, line)) {
        ++line_num;
        if (static_cast<int>(line.size()) > cfg.max_line_width) {
            violations.push_back({
                line_num, "line-width",
                "line is " + std::to_string(line.size()) +
                    " chars (max " + std::to_string(cfg.max_line_width) + ")",
                "error"
            });
        }
    }
    return violations;
}

// --- Code lines check ---

static std::vector<Violation> check_code_lines(
    const std::string& path,
    const Config& cfg
) {
    if (cfg.max_code_lines_per_file <= 0) return {};
    std::ifstream file(path);
    std::string line;
    int count = 0;

    while (std::getline(file, line)) {
        size_t start = line.find_first_not_of(" \t\r");
        if (start == std::string::npos) continue;
        if (line.compare(start, 2, "//") == 0) continue;
        ++count;
    }
    if (count > cfg.max_code_lines_per_file) {
        return {{
            1, "file-length",
            "file has " + std::to_string(count) +
                " code lines (max " + std::to_string(cfg.max_code_lines_per_file) + ")",
            "error"
        }};
    }
    return {};
}

// --- AST helpers ---

struct CheckContext {
    const Config* cfg;
    std::vector<Violation>* violations;
    bool is_test;
    int class_count;
    std::string file_path;
};

static int get_line(CXCursor cursor) {
    unsigned line = 0;
    clang_getSpellingLocation(
        clang_getCursorLocation(cursor),
        nullptr, &line, nullptr, nullptr
    );
    return static_cast<int>(line);
}

static std::string get_spelling(CXCursor cursor) {
    auto name = clang_getCursorSpelling(cursor);
    std::string result = clang_getCString(name);
    clang_disposeString(name);
    return result;
}

static bool cursor_in_file(CXCursor cursor, const std::string& file_path) {
    CXFile file = nullptr;
    clang_getSpellingLocation(
        clang_getCursorLocation(cursor),
        &file, nullptr, nullptr, nullptr
    );
    if (!file) return false;
    auto name = clang_getFileName(file);
    std::string cursor_path = clang_getCString(name);
    clang_disposeString(name);
    char* resolved = realpath(cursor_path.c_str(), nullptr);
    if (resolved) {
        cursor_path = resolved;
        free(resolved);
    }
    return cursor_path == file_path;
}

static int cursor_end_line(CXCursor cursor) {
    unsigned line = 0;
    clang_getSpellingLocation(
        clang_getRangeEnd(clang_getCursorExtent(cursor)),
        nullptr, &line, nullptr, nullptr
    );
    return static_cast<int>(line);
}

// --- Nesting depth ---

struct NestData { int max_depth; int current; };

static CXChildVisitResult nesting_visitor(
    CXCursor c, CXCursor, CXClientData ud
) {
    auto* d = static_cast<NestData*>(ud);
    auto kind = clang_getCursorKind(c);

    bool nests =
        kind == CXCursor_IfStmt ||
        kind == CXCursor_ForStmt ||
        kind == CXCursor_WhileStmt ||
        kind == CXCursor_DoStmt ||
        kind == CXCursor_SwitchStmt ||
        kind == CXCursor_CXXForRangeStmt;

    if (nests) {
        NestData child{d->current + 1, d->current + 1};
        clang_visitChildren(c, nesting_visitor, &child);
        if (child.max_depth > d->max_depth) {
            d->max_depth = child.max_depth;
        }
        return CXChildVisit_Continue;
    }

    NestData child{d->current, d->current};
    clang_visitChildren(c, nesting_visitor, &child);
    if (child.max_depth > d->max_depth) {
        d->max_depth = child.max_depth;
    }
    return CXChildVisit_Continue;
}

static int compute_nesting(CXCursor cursor) {
    NestData data{0, 0};
    clang_visitChildren(cursor, nesting_visitor, &data);
    return data.max_depth;
}

// --- Function checks ---

static void check_function(CXCursor cursor, CheckContext* ctx) {
    auto name = get_spelling(cursor);
    if (name.empty()) name = "(anonymous)";

    int start = get_line(cursor);
    int end = cursor_end_line(cursor);
    int length = end - start - 1;

    if (length > ctx->cfg->max_method_length) {
        ctx->violations->push_back({
            start, "method-length",
            name + " is " + std::to_string(length) +
                " lines (max " + std::to_string(ctx->cfg->max_method_length) + ")",
            "error"
        });
    }

    int params = clang_Cursor_getNumArguments(cursor);
    if (params > ctx->cfg->max_params) {
        ctx->violations->push_back({
            start, "param-count",
            name + " has " + std::to_string(params) +
                " parameters (max " + std::to_string(ctx->cfg->max_params) + ")",
            "error"
        });
    }

    int depth = compute_nesting(cursor);
    if (depth > ctx->cfg->max_nesting_depth) {
        ctx->violations->push_back({
            start, "nesting-depth",
            name + " has nesting depth " + std::to_string(depth) +
                " (max " + std::to_string(ctx->cfg->max_nesting_depth) + ")",
            "error"
        });
    }
}

// --- Public member check ---

static CXChildVisitResult member_visitor(
    CXCursor c, CXCursor parent, CXClientData ud
) {
    auto* ctx = static_cast<CheckContext*>(ud);
    auto kind = clang_getCursorKind(c);

    if (kind == CXCursor_FieldDecl) {
        auto access = clang_getCXXAccessSpecifier(c);
        if (access == CX_CXXPublic) {
            auto field_name = get_spelling(c);
            auto class_name = get_spelling(parent);
            ctx->violations->push_back({
                get_line(c), "no-public-members",
                class_name + "." + field_name +
                    ": public data members break encapsulation; use methods",
                "error"
            });
        }
    }
    return CXChildVisit_Continue;
}

// --- Main visitor ---

static CXChildVisitResult root_visitor(
    CXCursor c, CXCursor, CXClientData ud
) {
    auto* ctx = static_cast<CheckContext*>(ud);

    // Skip anything not in the file being checked
    if (!cursor_in_file(c, ctx->file_path)) return CXChildVisit_Continue;

    auto kind = clang_getCursorKind(c);

    // Check functions/methods
    if (kind == CXCursor_FunctionDecl ||
        kind == CXCursor_CXXMethod ||
        kind == CXCursor_Constructor) {
        if (clang_isCursorDefinition(c)) {
            check_function(c, ctx);
        }
    }

    // Count classes/structs and check public members
    if (kind == CXCursor_ClassDecl || kind == CXCursor_StructDecl) {
        if (clang_isCursorDefinition(c)) {
            ctx->class_count++;
            if (!ctx->is_test && ctx->cfg->forbid_public_members) {
                auto access = clang_getCXXAccessSpecifier(c);
                if (access != CX_CXXPrivate) {
                    clang_visitChildren(c, member_visitor, ctx);
                }
            }
        }
    }

    // Check raw new and C-style casts (non-test only)
    if (!ctx->is_test) {
        if (ctx->cfg->forbid_raw_new && kind == CXCursor_CXXNewExpr) {
            ctx->violations->push_back({
                get_line(c), "no-raw-new",
                "use smart pointers instead of raw new",
                "error"
            });
        }
        if (ctx->cfg->forbid_c_casts && kind == CXCursor_CStyleCastExpr) {
            ctx->violations->push_back({
                get_line(c), "no-c-cast",
                "use static_cast/dynamic_cast instead of C-style cast",
                "error"
            });
        }
        if (ctx->cfg->forbid_empty_catch && kind == CXCursor_CXXCatchStmt) {
            // Check if catch body (compound stmt) is empty
            struct ChildData { int stmt_count; };
            ChildData cd{0};
            clang_visitChildren(c, [](CXCursor child, CXCursor, CXClientData ud) {
                auto* d = static_cast<ChildData*>(ud);
                if (clang_getCursorKind(child) == CXCursor_CompoundStmt) {
                    struct BodyData { int count; };
                    BodyData bd{0};
                    clang_visitChildren(child, [](CXCursor, CXCursor, CXClientData ud2) {
                        static_cast<BodyData*>(ud2)->count++;
                        return CXChildVisit_Break;
                    }, &bd);
                    d->stmt_count = bd.count;
                }
                return CXChildVisit_Continue;
            }, &cd);
            if (cd.stmt_count == 0) {
                ctx->violations->push_back({
                    get_line(c), "no-empty-catch",
                    "empty catch block swallows errors; handle or log explicitly",
                    "error"
                });
            }
        }
        if (ctx->cfg->forbid_deep_qualified &&
            (kind == CXCursor_DeclRefExpr || kind == CXCursor_TypeRef)) {
            auto display = clang_getCursorDisplayName(c);
            std::string name = clang_getCString(display);
            clang_disposeString(display);
            int colons = 0;
            for (size_t i = 0; i + 1 < name.size(); ++i) {
                if (name[i] == ':' && name[i + 1] == ':') { ++colons; ++i; }
            }
            if (colons >= 2) {
                ctx->violations->push_back({
                    get_line(c), "no-deep-qualified",
                    name + ": use a using/namespace import instead of fully qualified name",
                    "error"
                });
            }
        }
    }

    return CXChildVisit_Recurse;
}

// --- Entry point ---

std::vector<Violation> check_file(
    const std::string& path,
    const Config& cfg
) {
    auto violations = check_line_width(path, cfg);
    auto code_violations = check_code_lines(path, cfg);
    violations.insert(violations.end(), code_violations.begin(), code_violations.end());

    bool is_test = path.find("_test.") != std::string::npos ||
                   path.find("/test/") != std::string::npos ||
                   path.find("/tests/") != std::string::npos;

    CXIndex index = clang_createIndex(0, 0);
    const char* args[] = {"-std=c++20", "-x", "c++"};
    CXTranslationUnit tu = clang_parseTranslationUnit(
        index, path.c_str(), args, 3, nullptr, 0, 0
    );

    if (!tu) {
        violations.push_back({1, "parse-error", "failed to parse " + path, "error"});
        clang_disposeIndex(index);
        return violations;
    }

    CheckContext ctx{&cfg, &violations, is_test, 0, ""};
    char* resolved = realpath(path.c_str(), nullptr);
    if (resolved) {
        ctx.file_path = resolved;
        free(resolved);
    } else {
        ctx.file_path = path;
    }
    clang_visitChildren(clang_getTranslationUnitCursor(tu), root_visitor, &ctx);

    if (cfg.max_classes_per_file > 0 && ctx.class_count > cfg.max_classes_per_file) {
        violations.push_back({
            1, "max-classes-per-file",
            "file has " + std::to_string(ctx.class_count) +
                " classes (max " + std::to_string(cfg.max_classes_per_file) + ")",
            "error"
        });
    }

    clang_disposeTranslationUnit(tu);
    clang_disposeIndex(index);
    return violations;
}
