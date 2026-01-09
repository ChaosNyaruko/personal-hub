#!/bin/bash

# Run tests and generate coverage profile
go test -coverprofile=coverage.out ./...

# Extract total coverage percentage
COVERAGE=$(go tool cover -func=coverage.out | grep total: | awk '{print $3}' | sed 's/%//')

# Determine color based on coverage
if (( $(echo "$COVERAGE < 50" | bc -l) )); then
    COLOR="red"
elif (( $(echo "$COVERAGE < 80" | bc -l) )); then
    COLOR="orange"
else
    COLOR="green"
fi

# Construct badge URL
BADGE_URL="https://img.shields.io/badge/coverage-${COVERAGE}%25-${COLOR}"

# Update README.md
# This sed command looks for the coverage badge line and replaces it
# It assumes the badge is in the format ![Coverage](...)
sed -i '' "s|!\[Coverage\](https://img.shields.io/badge/coverage-.*)|![Coverage]($BADGE_URL)|" README.md

echo "Coverage updated to $COVERAGE% ($COLOR)"
rm coverage.out
