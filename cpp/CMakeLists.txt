cmake_minimum_required(VERSION 3.20)
project(mcp-lint-cpp VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

# Find libclang
find_path(LIBCLANG_INCLUDE_DIR clang-c/Index.h
    HINTS
        /opt/homebrew/opt/llvm/include
        /usr/lib/llvm-18/include
        /usr/lib/llvm-17/include
        /Library/Developer/CommandLineTools/usr/include
        /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/include
        /usr/include
        /usr/local/include
)

find_library(LIBCLANG_LIBRARY clang
    HINTS
        /opt/homebrew/opt/llvm/lib
        /usr/lib/llvm-18/lib
        /usr/lib/llvm-17/lib
        /Library/Developer/CommandLineTools/usr/lib
        /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib
        /usr/lib
        /usr/local/lib
)

if(NOT LIBCLANG_INCLUDE_DIR OR NOT LIBCLANG_LIBRARY)
    message(FATAL_ERROR "libclang not found")
endif()

message(STATUS "libclang include: ${LIBCLANG_INCLUDE_DIR}")
message(STATUS "libclang library: ${LIBCLANG_LIBRARY}")

add_executable(mcp-lint-cpp
    src/main.cpp
    src/checker.cpp
    src/types.cpp
)

target_include_directories(mcp-lint-cpp PRIVATE
    ${LIBCLANG_INCLUDE_DIR}
    src
)

target_link_libraries(mcp-lint-cpp PRIVATE
    ${LIBCLANG_LIBRARY}
)

install(TARGETS mcp-lint-cpp DESTINATION bin)
