#pragma once

#include "types.h"
#include <string>
#include <vector>

[[nodiscard]] std::vector<Violation> check_file(
    const std::string& path,
    const Config& cfg
);
