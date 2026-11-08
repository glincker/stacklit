#!/usr/bin/env python3
"""Example stacklit adapter: turns repeated capitalised terms in text files
into concept nodes. It reads file paths on stdin (one per line) and prints one
JSON object per line. Replace extract() with OCR, speech-to-text or any local
tool; stacklit only cares about the JSONL contract (see README.md)."""
import collections
import json
import re
import subprocess
import sys

TEXT = {".txt", ".md", ".rst", ".csv"}
MIN_COUNT = 2  # a term must appear this often in one file to become a concept
TERM = re.compile(r"\b[A-Z][a-z]+(?:\s[A-Z][a-z]+)*\b")


def ext(path):
    i = path.rfind(".")
    return path[i:].lower() if i >= 0 else ""


def extract(path):
    if ext(path) in TEXT:
        with open(path, encoding="utf-8", errors="ignore") as fh:
            return fh.read()
    if ext(path) == ".pdf":
        try:  # optional: needs poppler's pdftotext on PATH
            return subprocess.run(["pdftotext", path, "-"], capture_output=True, text=True, timeout=30).stdout
        except (OSError, subprocess.SubprocessError):
            return ""
    return ""


def main():
    for line in sys.stdin:
        path = line.strip()
        if not path:
            continue
        try:
            text = extract(path)
        except OSError:
            continue
        counts = collections.Counter(TERM.findall(text))
        for term, n in sorted(counts.items()):
            if n < MIN_COUNT:
                continue
            cid = term.lower().replace(" ", "-")
            print(json.dumps({"type": "node", "id": cid, "name": term, "kind": "concept",
                              "summary": f"mentioned {n} times", "source_file": path}))
            print(json.dumps({"type": "edge", "from": cid, "to": path, "kind": "mentioned_in", "source_file": path}))


if __name__ == "__main__":
    main()
