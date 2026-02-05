#!/bin/bash
set -e

# Usage: ./fetch_custom_fields.sh
# Requires JIRA_URL and JIRA_PAT environment variables to be set.

if [ -z "$JIRA_URL" ] || [ -z "$JIRA_PAT" ]; then
    echo "Error: JIRA_URL and JIRA_PAT must be set."
    echo "Usage: export JIRA_URL=...; export JIRA_PAT=...; ./scripts/fetch_custom_fields.sh"
    exit 1
fi

# Clean JIRA_URL (remove trailing slash)
JIRA_URL="${JIRA_URL%/}"

echo "Fetching Fields from $JIRA_URL..." >&2

# The field endpoint returns a list of all fields, including custom ones.
RESPONSE=$(curl -s -H "Authorization: Bearer $JIRA_PAT" \
     -H "Accept: application/json" \
     "$JIRA_URL/rest/api/2/field")

# Check if curl failed (empty response)
if [ -z "$RESPONSE" ]; then
    echo "Error: Empty response from Jira." >&2
    exit 1
fi

# Setup formatting with jq if available
if command -v jq &> /dev/null; then
    echo "$RESPONSE" | jq .
else
    echo "$RESPONSE"
fi
