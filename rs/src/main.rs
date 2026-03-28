mod checker;
mod types;

use std::env;
use std::fs;
use std::process;

use types::Config;

fn main() {
    let args: Vec<String> = env::args().collect();

    if args.len() < 3 || args[1] != "check" {
        eprintln!("Usage: mcp-lint-rs check <file.rs> [--config-json <json>]");
        process::exit(2);
    }

    let path = &args[2];
    let cfg = parse_config(&args);

    let source = match fs::read_to_string(path) {
        Ok(s) => s,
        Err(e) => {
            eprintln!("error: {e}");
            process::exit(1);
        }
    };

    let violations = checker::check(&source, path, &cfg);
    let json = serde_json::to_string_pretty(&violations)
        .unwrap_or_else(|_| "[]".to_string());
    println!("{json}");

    if !violations.is_empty() {
        process::exit(1);
    }
}

fn parse_config(args: &[String]) -> Config {
    for i in 0..args.len() {
        if args[i] == "--config-json" {
            if let Some(json) = args.get(i + 1) {
                if let Ok(cfg) = serde_json::from_str(json) {
                    return cfg;
                }
            }
        }
    }
    Config::default()
}
