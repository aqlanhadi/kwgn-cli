package common

import (
	"testing"
	"time"
)

func TestCleanDecimal_SimpleNumber(t *testing.T) {
	result, err := CleanDecimal("123.45")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.String() != "123.45" {
		t.Errorf("Expected '123.45', got '%s'", result.String())
	}
}

func TestCleanDecimal_WithCommas(t *testing.T) {
	result, err := CleanDecimal("1,234.56")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.String() != "1234.56" {
		t.Errorf("Expected '1234.56', got '%s'", result.String())
	}
}

func TestCleanDecimal_WithCurrencySymbol(t *testing.T) {
	result, err := CleanDecimal("RM 1,234.56")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.String() != "1234.56" {
		t.Errorf("Expected '1234.56', got '%s'", result.String())
	}
}

func TestCleanDecimal_WithSuffix(t *testing.T) {
	result, err := CleanDecimal("100.00CR")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.String() != "100" {
		t.Errorf("Expected '100', got '%s'", result.String())
	}
}

func TestCleanDecimal_WithPrefix(t *testing.T) {
	result, err := CleanDecimal("BEGINNING BALANCE 500.00")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.String() != "500" {
		t.Errorf("Expected '500', got '%s'", result.String())
	}
}

func TestCleanDecimal_EmptyString(t *testing.T) {
	result, err := CleanDecimal("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.IsZero() {
		t.Errorf("Expected zero, got '%s'", result.String())
	}
}

func TestCleanDecimal_NoNumbers(t *testing.T) {
	result, err := CleanDecimal("ABC")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.IsZero() {
		t.Errorf("Expected zero, got '%s'", result.String())
	}
}

func TestCleanDecimal_NegativeSign(t *testing.T) {
	// Note: The current implementation strips non-numeric chars including minus
	// This test documents the current behavior
	result, err := CleanDecimal("-123.45")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// Minus sign is stripped, so result is positive
	if result.String() != "123.45" {
		t.Errorf("Expected '123.45', got '%s'", result.String())
	}
}

func TestCleanDecimal_LargeNumber(t *testing.T) {
	result, err := CleanDecimal("1,234,567.89")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.String() != "1234567.89" {
		t.Errorf("Expected '1234567.89', got '%s'", result.String())
	}
}

func TestFixDateYear_SameYear(t *testing.T) {
	txDate := time.Date(2024, 11, 15, 0, 0, 0, 0, time.Local)
	stmtDate := time.Date(2024, 11, 30, 0, 0, 0, 0, time.Local)

	result := FixDateYear(txDate, stmtDate)

	if result.Year() != 2024 {
		t.Errorf("Expected year 2024, got %d", result.Year())
	}
	if result.Month() != 11 {
		t.Errorf("Expected month 11, got %d", result.Month())
	}
}

func TestFixDateYear_PreviousYear(t *testing.T) {
	// Transaction in December, statement in January
	// Should adjust to previous year
	txDate := time.Date(0, 12, 15, 0, 0, 0, 0, time.Local)
	stmtDate := time.Date(2024, 1, 31, 0, 0, 0, 0, time.Local)

	result := FixDateYear(txDate, stmtDate)

	if result.Year() != 2023 {
		t.Errorf("Expected year 2023, got %d", result.Year())
	}
	if result.Month() != 12 {
		t.Errorf("Expected month 12, got %d", result.Month())
	}
	if result.Day() != 15 {
		t.Errorf("Expected day 15, got %d", result.Day())
	}
}

func TestFixDateYear_CurrentYear(t *testing.T) {
	// Transaction in October, statement in November
	// Should use statement year
	txDate := time.Date(0, 10, 20, 0, 0, 0, 0, time.Local)
	stmtDate := time.Date(2024, 11, 30, 0, 0, 0, 0, time.Local)

	result := FixDateYear(txDate, stmtDate)

	if result.Year() != 2024 {
		t.Errorf("Expected year 2024, got %d", result.Year())
	}
	if result.Month() != 10 {
		t.Errorf("Expected month 10, got %d", result.Month())
	}
}

func TestFixDateYear_SameMonth(t *testing.T) {
	// Transaction and statement in same month
	txDate := time.Date(0, 11, 10, 0, 0, 0, 0, time.Local)
	stmtDate := time.Date(2024, 11, 30, 0, 0, 0, 0, time.Local)

	result := FixDateYear(txDate, stmtDate)

	if result.Year() != 2024 {
		t.Errorf("Expected year 2024, got %d", result.Year())
	}
	if result.Month() != 11 {
		t.Errorf("Expected month 11, got %d", result.Month())
	}
}

func TestParseDate_ValidDate(t *testing.T) {
	result, err := ParseDate("02/01/06", "15/11/24")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Day() != 15 {
		t.Errorf("Expected day 15, got %d", result.Day())
	}
	if result.Month() != 11 {
		t.Errorf("Expected month 11, got %d", result.Month())
	}
	if result.Year() != 2024 {
		t.Errorf("Expected year 2024, got %d", result.Year())
	}
}

func TestParseDate_InvalidDate(t *testing.T) {
	_, err := ParseDate("02/01/06", "invalid")
	if err == nil {
		t.Error("Expected error for invalid date, got nil")
	}
}
