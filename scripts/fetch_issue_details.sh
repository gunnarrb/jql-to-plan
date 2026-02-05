#!/bin/bash
set -e

# Usage: ./fetch_issue_details.sh <JIRA_ID_1> [JIRA_ID_2 ...]
# Requires JIRA_URL and JIRA_PAT environment variables to be set.

if [ -z "$JIRA_URL" ] || [ -z "$JIRA_PAT" ]; then
    echo "Error: JIRA_URL and JIRA_PAT must be set."
    echo "Usage: export JIRA_URL=...; export JIRA_PAT=...; ./scripts/fetch_issue_details.sh <JIRA_ID>"
    exit 1
fi

if [ "$#" -eq 0 ]; then
    echo "Usage: $0 <JIRA_ID_1> [JIRA_ID_2 ...]"
    exit 1
fi

# Clean JIRA_URL (remove trailing slash)
JIRA_URL="${JIRA_URL%/}"

for ISSUE_KEY in "$@"; do
    echo "--------------------------------------------------------------------------------"
    echo "Fetching Issue: $ISSUE_KEY"
    
    RESPONSE=$(curl -s -H "Authorization: Bearer $JIRA_PAT" \
         -H "Accept: application/json" \
         "$JIRA_URL/rest/api/2/issue/$ISSUE_KEY")

    if [ -z "$RESPONSE" ]; then
        echo "Error: Empty response for $ISSUE_KEY"
        continue
    fi

    if command -v jq &> /dev/null; then
        # Check if the response contains errorMessages
        IS_ERROR=$(echo "$RESPONSE" | jq 'has("errorMessages")')
        if [ "$IS_ERROR" = "true" ]; then
             echo "Error fetching $ISSUE_KEY:"
             echo "$RESPONSE" | jq -r '.errorMessages[]'
        else
             # Extract and print relevant information
             echo "$RESPONSE" | jq '{
                 key: .key,
                 summary: .fields.summary,
                 status: .fields.status.name,
                 issueLinks: .fields.issuelinks | map({
                     id: .id,
                     type: .type.name,
                     outwardIssue: (.outwardIssue | {key: .key, summary: .fields.summary, status: .fields.status.name} // null),
                     inwardIssue: (.inwardIssue | {key: .key, summary: .fields.summary, status: .fields.status.name} // null)
                 })
             }'
        fi
    else
        # Fallback to printing raw JSON if jq is not installed
        echo "$RESPONSE"
    fi
    echo ""
done
