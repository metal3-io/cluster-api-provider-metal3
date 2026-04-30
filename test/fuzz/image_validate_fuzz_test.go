package fuzz

import (
	"strings"
	"testing"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// FuzzImageValidate tests the Image.Validate function with random inputs
// to discover edge cases, panics, or unexpected behaviors.
// This fuzz test aims to ensure that Image.Validate handles all possible
// string inputs gracefully without crashing, even with malformed URLs,
// special characters, or unusual input combinations.
func FuzzImageValidate(f *testing.F) {
	// Seed corpus with known valid and invalid cases
	// These provide starting points for the fuzzer to mutate

	// Valid cases
	f.Add("http://example.com/image.qcow2", "sha256:abc123", "qcow2", false)
	f.Add("https://example.com/image.qcow2", "http://example.com/checksum.sha256", "qcow2", false)
	f.Add("http://172.22.0.1/rhcos.qcow2", "f7600f7a274d974a236c4da5161265859c32da93a7c8de6a77d560378a1384ef", "raw", false)
	f.Add("http://172.22.0.1:8080/images/rhcos.qcow2", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "qcow2", false)
	f.Add("http://example.com/live.iso", "", "live-iso", true) // live-iso doesn't require checksum

	// Invalid/edge cases
	f.Add("", "", "", false)                                           // Empty values
	f.Add("not-a-url", "checksum", "", false)                          // Invalid URL
	f.Add("http://example.com/image", "", "", false)                   // Missing checksum
	f.Add("http://example.com", "http://:invalid", "", false)          // Malformed checksum URL
	f.Add("ftp://example.com/image", "checksum", "", false)            // Non-HTTP protocol
	f.Add("http://example.com/image", "https://", "", false)           // Incomplete checksum URL
	f.Add("http://", "checksum", "", false)                            // Incomplete URL
	f.Add("://example.com", "checksum", "", false)                     // Missing protocol
	f.Add("http://example.com/\x00", "checksum", "", false)            // Null byte
	f.Add("http://example.com/image", "http://\x00", "", false)        // Null byte in checksum
	f.Add(strings.Repeat("a", 10000), "checksum", "", false)           // Very long URL
	f.Add("http://example.com", strings.Repeat("a", 10000), "", false) // Very long checksum

	f.Fuzz(func(t *testing.T, url, checksum, diskFormat string, isLiveISO bool) {
		// Create Image struct with fuzzed inputs
		// In v1beta2: Checksum is *string, DiskFormat is string
		img := &infrav1.Image{
			URL: url,
		}

		// Set Checksum pointer
		if checksum != "" {
			img.Checksum = &checksum
		}

		// Set DiskFormat if provided (it's a string, not pointer in v1beta2)
		if diskFormat != "" {
			img.DiskFormat = diskFormat
		}

		// Override with live-iso format if specified
		if isLiveISO {
			img.DiskFormat = infrav1.LiveISODiskFormat
		}

		// Create a field path for validation errors
		basePath := field.NewPath("Spec", "Image")

		// The main assertion: Validate should never panic
		// It doesn't concern us if it returns errors (that's expected for invalid input),
		// but it should always complete without crashing
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Image.Validate panicked with input:\n  URL: %q\n  Checksum: %q\n  DiskFormat: %v\n  Panic: %v",
					url, checksum, diskFormat, r)
			}
		}()

		// Call the function under test
		errors := img.Validate(*basePath)

		// Basic sanity checks on the result
		if errors == nil {
			// If no errors returned, we expect the inputs to be valid
			// For valid inputs, URL should not be empty
			if img.URL == "" {
				t.Errorf("Validate returned nil errors for empty URL")
			}
		}

		// Ensure returned errors are properly structured
		for _, err := range errors {
			if err == nil {
				t.Error("Validate returned nil error in ErrorList")
			}
			// Ensure error has required fields
			if err != nil && err.Type == "" {
				t.Errorf("Validation error missing Type field: %v", err)
			}
		}

		// Additional invariant checks
		checkInvariants(t, img, errors)
	})
}

// checkInvariants verifies logical consistency of validation results.
func checkInvariants(t *testing.T, img *infrav1.Image, errors field.ErrorList) {
	t.Helper()

	// Invariant 1: Empty URL should always produce an error
	// When both URL and Checksum are empty, error may be on base field or URL field
	if img.URL == "" {
		hasURLError := false
		for _, err := range errors {
			if err.Field == "Spec.Image.URL" || err.Field == "Spec.Image" {
				hasURLError = true
				break
			}
		}
		if !hasURLError {
			t.Error("Empty URL should produce validation error")
		}
	}

	// Invariant 2: Non-live-iso images without checksum should error
	// In v1beta2: DiskFormat is string (not pointer)
	if img.DiskFormat != infrav1.LiveISODiskFormat {
		if (img.Checksum == nil || *img.Checksum == "") && img.URL != "" {
			hasChecksumError := false
			for _, err := range errors {
				if err.Field == "Spec.Image.Checksum" {
					hasChecksumError = true
					break
				}
			}
			if !hasChecksumError {
				t.Error("Missing checksum for non-live-iso image should produce validation error")
			}
		}
	}

	// Invariant 3: live-iso format should not require checksum
	// In v1beta2: DiskFormat is string (not pointer)
	if img.DiskFormat == infrav1.LiveISODiskFormat {
		for _, err := range errors {
			if err.Field == "Spec.Image.Checksum" && err.Type == field.ErrorTypeRequired {
				t.Error("live-iso format should not require checksum, but got Required error")
			}
		}
	}

	// Invariant 4: Checksum starting with http:// or https:// should be validated as invalid
	// against the checksum field/value using structured error fields.
	// In v1beta2: Checksum is *string (pointer)
	if img.Checksum != nil && *img.Checksum != "" {
		if strings.HasPrefix(*img.Checksum, "http://") || strings.HasPrefix(*img.Checksum, "https://") {
			for _, err := range errors {
				if err.Field == "Spec.Image.Checksum" && err.Type == field.ErrorTypeInvalid {
					if err.BadValue != *img.Checksum {
						t.Errorf("Checksum invalid error should reference the checksum value, got bad value: %#v", err.BadValue)
					}
				}
			}
		}
	}
}
