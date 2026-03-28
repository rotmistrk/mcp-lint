package dev.rotmistr.mcplint;

import com.google.gson.Gson;
import com.google.gson.GsonBuilder;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;

public final class Main {

    public static void main(String[] args) {
        if (args.length < 2 || !"check".equals(args[0])) {
            System.err.println("Usage: mcp-lint-java check <file.java> [--config-json <json>]");
            System.exit(2);
        }

        var path = args[1];
        var cfg = parseConfig(args);

        try {
            var source = Files.readString(Path.of(path));
            var violations = Checker.check(source, path, cfg);
            var gson = new GsonBuilder().setPrettyPrinting().create();
            System.out.println(gson.toJson(violations));
            System.exit(violations.isEmpty() ? 0 : 1);
        } catch (IOException e) {
            System.err.println("error: " + e.getMessage());
            System.exit(1);
        }
    }

    private static Config parseConfig(String[] args) {
        for (int i = 0; i < args.length - 1; i++) {
            if ("--config-json".equals(args[i])) {
                try {
                    return new Gson().fromJson(args[i + 1], Config.class);
                } catch (Exception ignored) {
                    break;
                }
            }
        }
        return new Config();
    }
}
