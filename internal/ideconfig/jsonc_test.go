package ideconfig

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// StripComments
// ---------------------------------------------------------------------------

func TestStripComments(t *testing.T) {
	tests := []struct {
		name  string
		input string
		// If wantExact is non-empty, assert got == wantExact.
		// Otherwise parse got as JSON and check wantJSON keys.
		wantExact string
		wantJSON  map[string]any
	}{
		{
			name:      "empty input returns empty string",
			input:     "",
			wantExact: "",
		},
		{
			name:      "no comments returns input unchanged",
			input:     `{"key": "value", "num": 42}`,
			wantExact: `{"key": "value", "num": 42}`,
		},
		{
			name:  "line comment is removed",
			input: "{\n  // this is a comment\n  \"key\": \"value\"\n}",
			wantJSON: map[string]any{
				"key": "value",
			},
		},
		{
			name:  "block comment is removed",
			input: "{\n  /* block comment */\n  \"key\": \"value\"\n}",
			wantJSON: map[string]any{
				"key": "value",
			},
		},
		{
			name:  "block comment spanning multiple lines",
			input: "{\n  /* this is\n     a multi-line\n     block comment */\n  \"key\": \"value\"\n}",
			wantJSON: map[string]any{
				"key": "value",
			},
		},
		{
			name:      "comments inside quoted strings are NOT stripped (line comment syntax)",
			input:     `{"key": "value // not a comment"}`,
			wantExact: `{"key": "value // not a comment"}`,
		},
		{
			name:      "comments inside quoted strings are NOT stripped (block comment syntax)",
			input:     `{"note": "/* not a comment */"}`,
			wantExact: `{"note": "/* not a comment */"}`,
		},
		{
			name:  "escaped quotes inside strings preserve comment-like text",
			input: `{"key": "val\"ue // still string"}`,
			wantJSON: map[string]any{
				"key": "val\"ue // still string",
			},
		},
		{
			name:  "trailing comma before closing brace is removed",
			input: `{"key": "value",}`,
			wantJSON: map[string]any{
				"key": "value",
			},
		},
		{
			name:  "trailing comma before closing bracket is removed",
			input: `{"arr": ["a", "b",]}`,
			wantJSON: map[string]any{
				"arr": []any{"a", "b"},
			},
		},
		{
			name:  "trailing comma with whitespace before closing brace",
			input: "{\n  \"a\": 1 ,  \n}",
			wantJSON: map[string]any{
				"a": float64(1),
			},
		},
		{
			name:  "comment at end of line after a value",
			input: "{\n  \"key\": \"value\" // inline comment\n}",
			wantJSON: map[string]any{
				"key": "value",
			},
		},
		{
			name: "mixed line and block comments in realistic Zed-like config",
			input: `{
  // UI settings
  "theme": "One Dark",
  "ui_font_size": 16, /* px */
  "buffer_font_size": 16,
  "context_servers": {
    /* MCP server for Chef */
    "chef-pkg": {
      "command": "/usr/local/bin/chef-pkg", // absolute path
      "args": ["serve"],
      "env": {} // empty env
    },
  },
  // Vim is optional
  "vim_mode": false,
}`,
			wantJSON: map[string]any{
				"theme":            "One Dark",
				"ui_font_size":     float64(16),
				"buffer_font_size": float64(16),
				"vim_mode":         false,
			},
		},
		{
			name:  "trailing comma after inline comment before closing brace",
			input: "{\n  \"a\": 1, // first\n  \"b\": 2, // last item\n}",
			wantJSON: map[string]any{
				"a": float64(1),
				"b": float64(2),
			},
		},
		{
			name:      "single slash in string is not treated as comment",
			input:     `{"path": "/usr/local/bin"}`,
			wantExact: `{"path": "/usr/local/bin"}`,
		},
		{
			name:  "escaped backslash before closing quote does not confuse parser",
			input: `{"path": "C:\\\\"}`,
			wantJSON: map[string]any{
				"path": `C:\\`,
			},
		},
		{
			name:  "unterminated block comment strips to end of input",
			input: `{"key": "value"} /* unterminated`,
			// The JSON object part should survive; the dangling comment is gone.
			wantExact: "", // handled by custom check below
		},
		{
			name:  "nested objects with comments and trailing commas",
			input: "{\n  \"servers\": {\n    \"a\": { \"cmd\": \"x\", }, // server a\n    \"b\": { \"cmd\": \"y\", }, /* server b */\n  },\n}",
			wantJSON: map[string]any{
				"servers": map[string]any{
					"a": map[string]any{"cmd": "x"},
					"b": map[string]any{"cmd": "y"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripComments(tt.input)

			// Special case: unterminated block comment
			if tt.name == "unterminated block comment strips to end of input" {
				if containsSubstring(got, "unterminated") {
					t.Errorf("unterminated block comment text should be stripped, got %q", got)
				}
				if !containsSubstring(got, `"key"`) {
					t.Errorf("JSON content before comment should survive, got %q", got)
				}
				return
			}

			if tt.wantExact != "" {
				if got != tt.wantExact {
					t.Errorf("StripComments()\n  got  = %q\n  want = %q", got, tt.wantExact)
				}
				return
			}

			// For empty-input case
			if tt.input == "" {
				if got != "" {
					t.Errorf("StripComments(\"\") = %q, want \"\"", got)
				}
				return
			}

			// Otherwise, the result must be valid JSON and match expected keys.
			if tt.wantJSON != nil {
				var m map[string]any
				if err := json.Unmarshal([]byte(got), &m); err != nil {
					t.Fatalf("result is not valid JSON: %v\nresult: %s", err, got)
				}
				for k, want := range tt.wantJSON {
					assertJSONValue(t, m, k, want)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ExtractPreamble
// ---------------------------------------------------------------------------

func TestExtractPreamble(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantPreamble string
		wantBody     string
	}{
		{
			name:         "empty input",
			input:        "",
			wantPreamble: "",
			wantBody:     "",
		},
		{
			name:         "input with no preamble starts with opening brace",
			input:        `{"key": "value"}`,
			wantPreamble: "",
			wantBody:     `{"key": "value"}`,
		},
		{
			name:         "bare empty object",
			input:        `{}`,
			wantPreamble: "",
			wantBody:     `{}`,
		},
		{
			name:         "single comment line before opening brace",
			input:        "// Zed settings\n{\"key\": \"value\"}",
			wantPreamble: "// Zed settings\n",
			wantBody:     "{\"key\": \"value\"}",
		},
		{
			name:         "multiple comment lines before opening brace",
			input:        "// Zed settings\n// More info\n// Third line\n{\n  \"key\": \"value\"\n}",
			wantPreamble: "// Zed settings\n// More info\n// Third line\n",
			wantBody:     "{\n  \"key\": \"value\"\n}",
		},
		{
			name:         "no opening brace returns entire input as preamble",
			input:        "// just some comments\n// no JSON here",
			wantPreamble: "// just some comments\n// no JSON here",
			wantBody:     "",
		},
		{
			name:         "whitespace before opening brace",
			input:        "\n  \n{}",
			wantPreamble: "\n  \n",
			wantBody:     "{}",
		},
		{
			name:         "brace inside comment line splits at first brace",
			input:        "// look a { brace\n{}",
			wantPreamble: "// look a ",
			wantBody:     "{ brace\n{}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPreamble, gotBody := ExtractPreamble(tt.input)
			if gotPreamble != tt.wantPreamble {
				t.Errorf("preamble:\n  got  = %q\n  want = %q", gotPreamble, tt.wantPreamble)
			}
			if gotBody != tt.wantBody {
				t.Errorf("body:\n  got  = %q\n  want = %q", gotBody, tt.wantBody)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Round-trip: ExtractPreamble then StripComments
// ---------------------------------------------------------------------------

func TestRoundTrip_PreambleThenStrip(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantPreamble string
		wantJSON     map[string]any
	}{
		{
			name: "full Zed settings file",
			input: `// Zed settings
{
  // font config
  "font_size": 14,
  "theme": "Solarized Dark", /* classic */
  "context_servers": {
    "chef-pkg": {
      "command": "/usr/local/bin/chef-pkg",
      "args": ["serve"],
      "env": {},
    },
  },
}`,
			wantPreamble: "// Zed settings\n",
			wantJSON: map[string]any{
				"font_size": float64(14),
				"theme":     "Solarized Dark",
			},
		},
		{
			name: "realistic Zed with multi-line preamble",
			input: `// Zed settings
//
// For information on how to configure Zed, see the Zed
// documentation: https://zed.dev/docs/configuring-zed
//
// To see all of Zed's default settings without any overrides,
// run the "zed: open default settings" command from the command
// palette or from "Zed > Settings > Open Default Settings".
{
  "theme": "One Dark",
  "ui_font_size": 16,
  "buffer_font_size": 16,
  "context_servers": {
    "chef-pkg": {
      "command": "/usr/local/bin/chef-pkg",
      "args": ["serve"],
      "env": {}
    }
  },
  // Some trailing comment
  "vim_mode": false,
}`,
			wantPreamble: "// Zed settings\n//\n// For information on how to configure Zed, see the Zed\n// documentation: https://zed.dev/docs/configuring-zed\n//\n// To see all of Zed's default settings without any overrides,\n// run the \"zed: open default settings\" command from the command\n// palette or from \"Zed > Settings > Open Default Settings\".\n",
			wantJSON: map[string]any{
				"theme":            "One Dark",
				"ui_font_size":     float64(16),
				"buffer_font_size": float64(16),
				"vim_mode":         false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preamble, body := ExtractPreamble(tt.input)
			if preamble != tt.wantPreamble {
				t.Errorf("preamble = %q, want %q", preamble, tt.wantPreamble)
			}

			cleaned := StripComments(body)
			var m map[string]any
			if err := json.Unmarshal([]byte(cleaned), &m); err != nil {
				t.Fatalf("round-trip produced invalid JSON: %v\ncleaned: %s", err, cleaned)
			}
			for k, want := range tt.wantJSON {
				assertJSONValue(t, m, k, want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// assertJSONValue checks a single key in the parsed map. It handles nested
// maps and slices for the test cases that need them.
func assertJSONValue(t *testing.T, m map[string]any, key string, want any) {
	t.Helper()
	got, ok := m[key]
	if !ok {
		t.Errorf("key %q missing from result", key)
		return
	}

	switch w := want.(type) {
	case map[string]any:
		gotMap, ok := got.(map[string]any)
		if !ok {
			t.Errorf("key %q: expected map, got %T", key, got)
			return
		}
		for k, v := range w {
			assertJSONValue(t, gotMap, k, v)
		}
	case []any:
		gotSlice, ok := got.([]any)
		if !ok {
			t.Errorf("key %q: expected slice, got %T", key, got)
			return
		}
		if len(gotSlice) != len(w) {
			t.Errorf("key %q: slice length = %d, want %d", key, len(gotSlice), len(w))
			return
		}
		for i, v := range w {
			if gotSlice[i] != v {
				t.Errorf("key %q[%d] = %v, want %v", key, i, gotSlice[i], v)
			}
		}
	default:
		if got != want {
			t.Errorf("key %q = %v (%T), want %v (%T)", key, got, got, want, want)
		}
	}
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
