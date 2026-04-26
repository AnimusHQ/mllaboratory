package env

import (
	"testing"
	"time"
)

func TestString_Default(t *testing.T) {
	t.Setenv("ENV_STRING_DEFAULT", "")
	got := String("ENV_STRING_DOES_NOT_EXIST", "fallback")
	if got != "fallback" {
		t.Fatalf("String()=%q, want fallback", got)
	}
}

func TestString_Override(t *testing.T) {
	t.Setenv("ENV_STRING_KEY", "value")
	got := String("ENV_STRING_KEY", "fallback")
	if got != "value" {
		t.Fatalf("String()=%q, want value", got)
	}
}

func TestDuration_Default(t *testing.T) {
	got, err := Duration("ENV_DURATION_DOES_NOT_EXIST", 5*time.Second)
	if err != nil {
		t.Fatalf("Duration() err=%v", err)
	}
	if got != 5*time.Second {
		t.Fatalf("Duration()=%v, want 5s", got)
	}
}

func TestDuration_Override(t *testing.T) {
	t.Setenv("ENV_DURATION_KEY", "250ms")
	got, err := Duration("ENV_DURATION_KEY", 5*time.Second)
	if err != nil {
		t.Fatalf("Duration() err=%v", err)
	}
	if got != 250*time.Millisecond {
		t.Fatalf("Duration()=%v, want 250ms", got)
	}
}

func TestDuration_Invalid(t *testing.T) {
	t.Setenv("ENV_DURATION_KEY_INVALID", "not-a-duration")
	_, err := Duration("ENV_DURATION_KEY_INVALID", 5*time.Second)
	if err == nil {
		t.Fatalf("Duration() expected error")
	}
}

func TestBool_Default(t *testing.T) {
	got, err := Bool("ENV_BOOL_DOES_NOT_EXIST", true)
	if err != nil {
		t.Fatalf("Bool() err=%v", err)
	}
	if got != true {
		t.Fatalf("Bool()=%v, want true", got)
	}
}

func TestBool_Override(t *testing.T) {
	t.Setenv("ENV_BOOL_KEY", "false")
	got, err := Bool("ENV_BOOL_KEY", true)
	if err != nil {
		t.Fatalf("Bool() err=%v", err)
	}
	if got != false {
		t.Fatalf("Bool()=%v, want false", got)
	}
}

func TestBool_Invalid(t *testing.T) {
	t.Setenv("ENV_BOOL_KEY_INVALID", "nope")
	_, err := Bool("ENV_BOOL_KEY_INVALID", false)
	if err == nil {
		t.Fatalf("Bool() expected error")
	}
}

func TestInt_Default(t *testing.T) {
	got, err := Int("ENV_INT_DOES_NOT_EXIST", 42)
	if err != nil {
		t.Fatalf("Int() err=%v", err)
	}
	if got != 42 {
		t.Fatalf("Int()=%v, want 42", got)
	}
}

func TestInt_Override(t *testing.T) {
	t.Setenv("ENV_INT_KEY", "7")
	got, err := Int("ENV_INT_KEY", 42)
	if err != nil {
		t.Fatalf("Int() err=%v", err)
	}
	if got != 7 {
		t.Fatalf("Int()=%v, want 7", got)
	}
}

func TestInt_Invalid(t *testing.T) {
	t.Setenv("ENV_INT_KEY_INVALID", "nope")
	_, err := Int("ENV_INT_KEY_INVALID", 42)
	if err == nil {
		t.Fatalf("Int() expected error")
	}
}
