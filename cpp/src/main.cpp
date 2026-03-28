#include "checker.h"
#include "types.h"

#include <iostream>
#include <string>

int main(int argc, char* argv[]) {
    if (argc < 3 || std::string(argv[1]) != "check") {
        std::cerr << "Usage: mcp-lint-cpp check <file> [--config-json <json>]\n";
        return 2;
    }

    std::string path = argv[2];
    Config cfg;

    for (int i = 3; i < argc - 1; ++i) {
        if (std::string(argv[i]) == "--config-json") {
            cfg = parse_config_json(argv[i + 1]);
        }
    }

    auto violations = check_file(path, cfg);
    std::cout << violations_to_json(violations) << "\n";

    return violations.empty() ? 0 : 1;
}
