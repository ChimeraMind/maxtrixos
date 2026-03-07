package config

import (
	"testing"
)

func TestFilterEnvKey(t *testing.T) {
	t.Run("RemovesSingleMatch", func(t *testing.T) {
		env := []string{"FOO=bar", "BAZ=qux"}
		result := FilterEnvKey(env, "FOO")
		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}
		if result[0] != "BAZ=qux" {
			t.Errorf("expected BAZ=qux, got %s", result[0])
		}
	})

	t.Run("RemovesMultipleMatches", func(t *testing.T) {
		env := []string{"FOO=bar", "FOO=baz", "OTHER=val"}
		result := FilterEnvKey(env, "FOO")
		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}
		if result[0] != "OTHER=val" {
			t.Errorf("expected OTHER=val, got %s", result[0])
		}
	})

	t.Run("NoMatch", func(t *testing.T) {
		env := []string{"FOO=bar", "BAZ=qux"}
		result := FilterEnvKey(env, "MISSING")
		if len(result) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(result))
		}
		if result[0] != "FOO=bar" || result[1] != "BAZ=qux" {
			t.Errorf("unexpected result: %v", result)
		}
	})

	t.Run("EmptyEnv", func(t *testing.T) {
		result := FilterEnvKey([]string{}, "FOO")
		if len(result) != 0 {
			t.Fatalf("expected 0 entries, got %d", len(result))
		}
	})

	t.Run("NilEnv", func(t *testing.T) {
		result := FilterEnvKey(nil, "FOO")
		if len(result) != 0 {
			t.Fatalf("expected 0 entries, got %d", len(result))
		}
	})

	t.Run("EmptyKey", func(t *testing.T) {
		env := []string{"=empty", "FOO=bar"}
		result := FilterEnvKey(env, "")
		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}
		if result[0] != "FOO=bar" {
			t.Errorf("expected FOO=bar, got %s", result[0])
		}
	})

	t.Run("PrefixNotConfused", func(t *testing.T) {
		env := []string{"FOO=bar", "FOOBAR=baz", "FOO_OTHER=qux"}
		result := FilterEnvKey(env, "FOO")
		if len(result) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(result))
		}
		if result[0] != "FOOBAR=baz" || result[1] != "FOO_OTHER=qux" {
			t.Errorf("unexpected result: %v", result)
		}
	})

	t.Run("OriginalUnmodified", func(t *testing.T) {
		env := []string{"FOO=bar", "BAZ=qux"}
		_ = FilterEnvKey(env, "FOO")
		if len(env) != 2 || env[0] != "FOO=bar" || env[1] != "BAZ=qux" {
			t.Errorf("original env was modified: %v", env)
		}
	})

	t.Run("AllRemoved", func(t *testing.T) {
		env := []string{"FOO=1", "FOO=2", "FOO=3"}
		result := FilterEnvKey(env, "FOO")
		if len(result) != 0 {
			t.Fatalf("expected 0 entries, got %d", len(result))
		}
	})
}
