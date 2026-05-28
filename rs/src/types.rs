use serde::{Deserialize, Serialize};

/// A single standards violation.
#[derive(Debug, Serialize, Deserialize)]
pub struct Violation {
    pub line: usize,
    pub rule: String,
    pub message: String,
    pub severity: String,
}

/// Lint configuration passed from the MCP server.
#[derive(Debug, Deserialize)]
pub struct Config {
    #[serde(default = "default_method_length")]
    pub max_method_length: usize,
    #[serde(default = "default_nesting_depth")]
    pub max_nesting_depth: usize,
    #[serde(default = "default_line_width")]
    pub max_line_width: usize,
    #[serde(default = "default_params")]
    pub max_params: usize,
    #[serde(default = "default_consecutive")]
    #[allow(dead_code)]
    pub max_consecutive_same_type: usize,
    #[serde(default = "default_code_lines")]
    pub max_code_lines_per_file: usize,
    #[serde(default)]
    pub rust: RustConfig,
}

#[derive(Debug, Deserialize)]
pub struct RustConfig {
    #[serde(default = "default_true")]
    pub forbid_unwrap: bool,
    #[serde(default = "default_true")]
    pub forbid_expect: bool,
    #[serde(default = "default_true")]
    pub forbid_panic: bool,
    #[serde(default = "default_true")]
    pub forbid_pub_fields: bool,
    #[serde(default = "default_true")]
    pub forbid_deep_path: bool,
    #[serde(default)]
    pub max_structs_per_file: usize,
}

impl Default for RustConfig {
    fn default() -> Self {
        Self {
            forbid_unwrap: true,
            forbid_expect: true,
            forbid_panic: true,
            forbid_pub_fields: true,
            forbid_deep_path: true,
            max_structs_per_file: 1,
        }
    }
}

impl Default for Config {
    fn default() -> Self {
        Self {
            max_method_length: 40,
            max_nesting_depth: 3,
            max_line_width: 120,
            max_params: 7,
            max_consecutive_same_type: 2,
            max_code_lines_per_file: 240,
            rust: RustConfig::default(),
        }
    }
}

fn default_method_length() -> usize { 40 }
fn default_nesting_depth() -> usize { 3 }
fn default_line_width() -> usize { 120 }
fn default_params() -> usize { 7 }
fn default_consecutive() -> usize { 2 }
fn default_code_lines() -> usize { 240 }
fn default_true() -> bool { true }
