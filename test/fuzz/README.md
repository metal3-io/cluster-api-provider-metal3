# Fuzzing

## Overview

Fuzz tests in this directory target critical validation and parsing functions
in CAPM3 that handle user-provided data. The goal is to ensure these functions
handle all possible inputs gracefully without crashing.

## Running Fuzz Tests

### Quick Start with Make

The easiest way to run fuzz tests is using Make targets:

```bash
# Run fuzz tests as regression tests (using seed corpus only, fast)
make fuzz

# Run all fuzz tests with fuzzing enabled (default: 30 seconds each)
make fuzz-run

# Run all fuzz tests with custom duration (e.g., 5 minutes each)
make fuzz-run FUZZ_TIME=5m
```

### Running Individual Fuzz Tests

To run a specific fuzz test directly:

```bash
# Run a specific fuzz test with fuzzing enabled
cd test/fuzz && go test -fuzz=FuzzImageValidate -fuzztime=30s

# Run with longer duration to find more edge cases
cd test/fuzz && go test -fuzz=FuzzImageValidate -fuzztime=5m

# Run only as regression test (no fuzzing, just seed corpus)
cd test/fuzz && go test -run=FuzzImageValidate
```

## Crash Corpus and Regression Testing

When fuzzing discovers a crash or failure, Go automatically saves the failing
input to `testdata/fuzz/<FuzzTestName>/` in the test directory. These crash
files should be committed to the repository:

```bash
git add test/fuzz/testdata/
git commit -m "Add fuzz crash corpus"
```

Once committed, these crashes are automatically replayed as regression tests
when running `make fuzz` (or `go test -run='^Fuzz'` to run all fuzz tests, or
`go test -run=FuzzImageValidate` for a specific test without fuzzing).

## Resources

- [Go Fuzzing Documentation](https://go.dev/doc/fuzz/)
- [Go Fuzzing Tutorial](https://go.dev/security/fuzz/)
