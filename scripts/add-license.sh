#!/bin/bash

# Script to add or update license header in Go files
# Usage: ./scripts/add-license.sh

set -e

LICENSE_HEADER=".license-header.txt"
LICENSE_TEXT=$(cat "$LICENSE_HEADER")

# Function to check if file has the correct license header (with correct year)
has_correct_license() {
    local file=$1
    # Check if file has "Copyright 2025 The HAMi Authors"
    if head -n 20 "$file" | grep -q "Copyright 2025.*The HAMi Authors"; then
        return 0
    fi
    return 1
}

# Function to remove old license header
remove_old_license() {
    local file=$1
    local temp_file=$(mktemp)
    local in_comment=false
    local skip_until_empty=false
    
    while IFS= read -r line || [ -n "$line" ]; do
        if [[ "$line" =~ ^/\* ]]; then
            in_comment=true
            skip_until_empty=true
            continue
        elif [[ "$in_comment" == true ]]; then
            if [[ "$line" =~ \*/ ]]; then
                in_comment=false
                skip_until_empty=true
                continue
            fi
            continue
        elif [[ "$skip_until_empty" == true ]]; then
            if [[ -z "$line" ]]; then
                skip_until_empty=false
            else
                continue
            fi
        fi
        echo "$line"
    done < "$file" > "$temp_file"
    
    mv "$temp_file" "$file"
}

# Find all Go files excluding vendor and .git directories
find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -path "./bin/*" | while read -r file; do
    if has_correct_license "$file"; then
        echo "✓ $file already has correct license header"
        continue
    fi
    
    # Remove old license if exists
    if head -n 1 "$file" | grep -q "^/\*"; then
        remove_old_license "$file"
    fi
    
    # Add new license header
    {
        echo "$LICENSE_TEXT"
        echo ""
        cat "$file"
    } > "$file.tmp"
    mv "$file.tmp" "$file"
    echo "✓ Updated license header in $file"
done

echo "License header update completed!"
