#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
import json
import sys
import os

def inventory(file_path):
    if not os.path.exists(file_path):
        print(f"Error: {file_path} not found", file=sys.stderr)
        sys.exit(1)
        
    with open(file_path, 'r') as f:
        data = json.load(f)
        
    threads = data.get('data', {}).get('repository', {}).get('pullRequest', {}).get('reviewThreads', {}).get('nodes', [])
    
    print(f"{ 'THREAD ID':<20} {'PATH':<30} {'LINE':<5} {'STATUS':<10} {'COMMENT'}")
    print("-" * 100)
    
    for t in threads:
        if t['isOutdated'] or t['isResolved']:
            continue
            
        tid = t['id']
        path = t['path']
        line = t['line']
        body = t['comments']['nodes'][0]['body'].replace('\n', ' ')[:50]
        
        print(f"{tid:<20} {path:<30} {line:<5} {'OPEN':<10} {body}")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: uv run pr.py <threads.json>", file=sys.stderr)
        sys.exit(1)
    inventory(sys.argv[1])
