#include "checker.h"

#include <clang-c/Index.h>
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

// --- AST helpers ---

struct CheckContext {
    const Config* cfg;
    std::vector<Violation>* violations;
    bool is_test;
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

// --- Main visitor ---

static CXChildVisitResult root_visitor(
    CXCursor c, CXCursor, CXClientData ud
) {
    auto* ctx = static_cast<CheckContext*>(ud);
    auto kind = clang_getCursorKind(c);

    // Check functions/methods
    if (kind == CXCursor_FunctionDecl ||
        kind == CXCursor_CXXMethod ||
        kind == CXCursor_Constructor) {
        if (clang_isCursorDefinition(c)) {
            check_function(c, ctx);
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
    }

    return CXChildVisit_Recurse;
}

// --- Entry point ---

std::vector<Violation> check_file(
    const std::string& path,
    const Config& cfg
) {
    auto violations = check_line_width(path, cfg);

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

    CheckContext ctx{&cfg, &violations, is_test};
    clang_visitChildren(clang_getTranslationUnitCursor(tu), root_visitor, &ctx);

    clang_disposeTranslationUnit(tu);
    clang_disposeIndex(index);
    return violations;
}
