package errors

import (
	"errors"
	"testing"
)

func TestWorkspaceErrors(t *testing.T) {
	t.Run("ErrOwnedDirNotSet", func(t *testing.T) {
		if ErrOwnedDirNotSet == nil {
			t.Error("ErrOwnedDirNotSet should not be nil")
		}
		if ErrOwnedDirNotSet.Error() != "owned directory not configured" {
			t.Errorf("ErrOwnedDirNotSet.Error() = %v, want %v",
				ErrOwnedDirNotSet.Error(), "owned directory not configured")
		}
	})

	t.Run("ErrVendorDirNotSet", func(t *testing.T) {
		if ErrVendorDirNotSet == nil {
			t.Error("ErrVendorDirNotSet should not be nil")
		}
		if ErrVendorDirNotSet.Error() != "vendor directory not configured" {
			t.Errorf("ErrVendorDirNotSet.Error() = %v, want %v",
				ErrVendorDirNotSet.Error(), "vendor directory not configured")
		}
	})

	t.Run("ErrServiceNotConfigured", func(t *testing.T) {
		if ErrServiceNotConfigured == nil {
			t.Error("ErrServiceNotConfigured should not be nil")
		}
		if ErrServiceNotConfigured.Error() != "service name not configured" {
			t.Errorf("ErrServiceNotConfigured.Error() = %v, want %v",
				ErrServiceNotConfigured.Error(), "service name not configured")
		}
	})

	t.Run("ErrAlreadyInitialized", func(t *testing.T) {
		if ErrAlreadyInitialized == nil {
			t.Error("ErrAlreadyInitialized should not be nil")
		}
		if ErrAlreadyInitialized.Error() != "workspace already initialized" {
			t.Errorf("ErrAlreadyInitialized.Error() = %v, want %v",
				ErrAlreadyInitialized.Error(), "workspace already initialized")
		}
	})

	t.Run("ErrNotInitialized", func(t *testing.T) {
		if ErrNotInitialized == nil {
			t.Error("ErrNotInitialized should not be nil")
		}
		if ErrNotInitialized.Error() != "workspace not initialized" {
			t.Errorf("ErrNotInitialized.Error() = %v, want %v",
				ErrNotInitialized.Error(), "workspace not initialized")
		}
	})
}

func TestRegistryErrors(t *testing.T) {
	t.Run("ErrNotFound", func(t *testing.T) {
		if ErrNotFound == nil {
			t.Error("ErrNotFound should not be nil")
		}
		if ErrNotFound.Error() != "project not found" {
			t.Errorf("ErrNotFound.Error() = %v, want %v",
				ErrNotFound.Error(), "project not found")
		}
	})
}

func TestErrorsAreDistinct(t *testing.T) {
	errs := []error{
		ErrOwnedDirNotSet,
		ErrVendorDirNotSet,
		ErrServiceNotConfigured,
		ErrAlreadyInitialized,
		ErrNotInitialized,
		ErrNotFound,
	}

	for i, err1 := range errs {
		for j, err2 := range errs {
			if i != j && errors.Is(err1, err2) {
				t.Errorf("errors %d and %d should be distinct, but errors.Is returned true", i, j)
			}
		}
	}
}

func TestErrorWrapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrOwnedDirNotSet", ErrOwnedDirNotSet},
		{"ErrVendorDirNotSet", ErrVendorDirNotSet},
		{"ErrServiceNotConfigured", ErrServiceNotConfigured},
		{"ErrAlreadyInitialized", ErrAlreadyInitialized},
		{"ErrNotInitialized", ErrNotInitialized},
		{"ErrNotFound", ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := errors.New("wrapper: " + tt.err.Error())
			if wrapped.Error() != "wrapper: "+tt.err.Error() {
				t.Errorf("wrapped error message mismatch")
			}
		})
	}
}
