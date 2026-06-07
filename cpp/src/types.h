#pragma once

#include <string>
#include <vector>

struct Violation {
    int line;
    std::string rule;
    std::string message;
    std::string severity;
};

struct Config {
    int max_method_length = 40;
    int max_nesting_depth = 3;
    int max_line_width = 120;
    int max_params = 7;
    int max_consecutive_same_type = 2;
    int max_code_lines_per_file = 240;
    bool forbid_raw_new = true;
    bool forbid_c_casts = true;
    bool forbid_public_members = true;
    bool forbid_empty_catch = true;
    bool forbid_deep_qualified = true;
    bool forbid_mutable_getters = true;
    int max_classes_per_file = 1;
};

[[nodiscard]] std::string violations_to_json(const std::vector<Violation>& violations);
[[nodiscard]] Config parse_config_json(const std::string& json);
