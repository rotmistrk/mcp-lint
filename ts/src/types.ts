export interface Violation {
  line: number;
  rule: string;
  message: string;
  severity: "error" | "warning";
}

export interface Config {
  max_method_length: number;
  max_nesting_depth: number;
  max_line_width: number;
  max_params: number;
  max_consecutive_same_type: number;
  max_code_lines_per_file: number;
  typescript: {
    forbid_any: boolean;
    forbid_class_components: boolean;
    forbid_wait_for_timeout: boolean;
    forbid_public_properties: boolean;
    forbid_empty_catch: boolean;
    forbid_deep_access: boolean;
    forbid_mutable_getters: boolean;
    forbid_mid_chain_optional: boolean;
    max_classes_per_file: number;
  };
}

export const defaults: Config = {
  max_method_length: 40,
  max_nesting_depth: 3,
  max_line_width: 120,
  max_params: 7,
  max_consecutive_same_type: 2,
  max_code_lines_per_file: 240,
  typescript: {
    forbid_any: true,
    forbid_class_components: true,
    forbid_wait_for_timeout: true,
    forbid_public_properties: true,
    forbid_empty_catch: true,
    forbid_deep_access: true,
    forbid_mutable_getters: true,
    forbid_mid_chain_optional: true,
    max_classes_per_file: 1,
  },
};
