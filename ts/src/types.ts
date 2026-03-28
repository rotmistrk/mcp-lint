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
  typescript: {
    forbid_any: boolean;
    forbid_class_components: boolean;
    forbid_wait_for_timeout: boolean;
  };
}

export const defaults: Config = {
  max_method_length: 40,
  max_nesting_depth: 3,
  max_line_width: 120,
  max_params: 7,
  max_consecutive_same_type: 2,
  typescript: {
    forbid_any: true,
    forbid_class_components: true,
    forbid_wait_for_timeout: true,
  },
};
