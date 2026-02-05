#!/bin/bash
set -eo pipefail

# Build flags array
FLAGS=""

if [ "$INPUT_RECURSIVE" = "true" ]; then
  FLAGS="$FLAGS -r"
fi

if [ "$INPUT_FAIL_FAST" = "true" ]; then
  FLAGS="$FLAGS -x"
fi

# Track exit code to fail the action if tests fail
EXIT_CODE=0

# Run with GitHub annotations for PR comments
if [ "$INPUT_ANNOTATIONS" = "true" ]; then
  echo "Running tests with GitHub annotations..."
  # shellcheck disable=SC2086
  skytest -github $FLAGS "$INPUT_PATH" || EXIT_CODE=$?
fi

# Run with markdown summary for job summary
if [ "$INPUT_SUMMARY" = "true" ]; then
  echo "Generating job summary..."
  # shellcheck disable=SC2086
  skytest -markdown $FLAGS "$INPUT_PATH" >>"$GITHUB_STEP_SUMMARY" 2>/dev/null || true
fi

# Run with coverage if requested
if [ "$INPUT_COVERAGE" = "true" ]; then
  echo "Collecting coverage..."
  # shellcheck disable=SC2086
  skytest --coverage --coverprofile=coverage.json $FLAGS "$INPUT_PATH" 2>/dev/null || true

  # Extract coverage percentage if file exists
  if [ -f coverage.json ]; then
    COVERAGE=$(jq -r '.percentage // 0' coverage.json 2>/dev/null || echo "0")
    echo "coverage=$COVERAGE" >>"$GITHUB_OUTPUT"

    # Check coverage threshold
    THRESHOLD="${INPUT_COVERAGE_THRESHOLD:-0}"
    if [ "$THRESHOLD" != "0" ]; then
      COVERAGE_INT=${COVERAGE%.*}
      THRESHOLD_INT=${THRESHOLD%.*}
      if [ "$COVERAGE_INT" -lt "$THRESHOLD_INT" ]; then
        echo "::error::Coverage ${COVERAGE}% is below threshold ${THRESHOLD}%"
        EXIT_CODE=1
      fi
    fi
  fi
fi

# Get test counts from JSON output for action outputs
echo "Extracting test results..."
# shellcheck disable=SC2086
RESULT=$(skytest -json $FLAGS "$INPUT_PATH" 2>/dev/null || echo '{"passed":0,"failed":0}')
PASSED=$(echo "$RESULT" | jq -r '.passed // 0')
FAILED=$(echo "$RESULT" | jq -r '.failed // 0')

echo "passed=$PASSED" >>"$GITHUB_OUTPUT"
echo "failed=$FAILED" >>"$GITHUB_OUTPUT"

# Exit with original test exit code
exit $EXIT_CODE
