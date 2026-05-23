package dev.rotmistr.mcplint;

public final class Config {
    public int max_method_length = 40;
    public int max_nesting_depth = 3;
    public int max_line_width = 120;
    public int max_params = 7;
    public int max_consecutive_same_type = 2;
    public int max_code_lines_per_file = 240;
    public JavaConfig java = new JavaConfig();

    public static final class JavaConfig {
        public boolean forbid_raw_types = true;
        public boolean forbid_public_fields = true;
        public int max_classes_per_file = 1;
    }
}
