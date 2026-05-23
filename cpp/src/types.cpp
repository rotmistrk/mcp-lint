#include "types.h"
#include <sstream>
#include <cstdlib>

static std::string escape_json(const std::string& s) {
    std::string out;
    out.reserve(s.size() + 10);
    for (char c : s) {
        switch (c) {
            case '"':  out += "\\\""; break;
            case '\\': out += "\\\\"; break;
            case '\n': out += "\\n";  break;
            default:   out += c;
        }
    }
    return out;
}

std::string violations_to_json(const std::vector<Violation>& violations) {
    std::ostringstream os;
    os << "[\n";
    for (size_t i = 0; i < violations.size(); ++i) {
        const auto& v = violations[i];
        os << "  {\"line\": " << v.line
           << ", \"rule\": \"" << escape_json(v.rule)
           << "\", \"message\": \"" << escape_json(v.message)
           << "\", \"severity\": \"" << escape_json(v.severity)
           << "\"}";
        if (i + 1 < violations.size()) os << ",";
        os << "\n";
    }
    os << "]";
    return os.str();
}

// Minimal JSON parsing — just extract known fields from flat config
static int json_int(const std::string& json, const char* key, int def) {
    auto pos = json.find(key);
    if (pos == std::string::npos) return def;
    pos = json.find(':', pos);
    if (pos == std::string::npos) return def;
    return std::atoi(json.c_str() + pos + 1);
}

static bool json_bool(const std::string& json, const char* key, bool def) {
    auto pos = json.find(key);
    if (pos == std::string::npos) return def;
    pos = json.find(':', pos);
    if (pos == std::string::npos) return def;
    auto rest = json.substr(pos + 1, 10);
    if (rest.find("true") != std::string::npos) return true;
    if (rest.find("false") != std::string::npos) return false;
    return def;
}

Config parse_config_json(const std::string& json) {
    Config cfg;
    cfg.max_method_length = json_int(json, "max_method_length", 40);
    cfg.max_nesting_depth = json_int(json, "max_nesting_depth", 3);
    cfg.max_line_width = json_int(json, "max_line_width", 120);
    cfg.max_params = json_int(json, "max_params", 7);
    cfg.max_consecutive_same_type = json_int(json, "max_consecutive_same_type", 2);
    cfg.max_code_lines_per_file = json_int(json, "max_code_lines_per_file", 240);
    cfg.forbid_raw_new = json_bool(json, "forbid_raw_new", true);
    cfg.forbid_c_casts = json_bool(json, "forbid_c_casts", true);
    cfg.forbid_public_members = json_bool(json, "forbid_public_members", true);
    cfg.max_classes_per_file = json_int(json, "max_classes_per_file", 1);
    return cfg;
}
