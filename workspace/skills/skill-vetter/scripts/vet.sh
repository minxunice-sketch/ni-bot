#!/bin/sh
set -e

REPO="$1"
if [ -z "$REPO" ]; then
  exit 1
fi

python3 - "$REPO" << 'PY'
import base64
import json
import os
import re
import sys
import urllib.request

repo = sys.argv[1].strip()
token = os.environ.get("GITHUB_TOKEN", "").strip()

def req_json(url: str):
  headers = {
    "User-Agent": "NiBot-Agent",
    "Accept": "application/vnd.github+json",
  }
  if token:
    headers["Authorization"] = "Bearer " + token
  r = urllib.request.Request(url, headers=headers)
  with urllib.request.urlopen(r, timeout=20) as resp:
    return json.loads(resp.read().decode("utf-8"))

def readme_text(repo: str) -> str:
  try:
    obj = req_json(f"https://api.github.com/repos/{repo}/contents/README.md")
  except Exception:
    return ""
  if not isinstance(obj, dict):
    return ""
  content = obj.get("content") or ""
  enc = (obj.get("encoding") or "").lower()
  if enc != "base64" or not content:
    return ""
  try:
    data = base64.b64decode(re.sub(r"\s+", "", content))
    return data.decode("utf-8", errors="replace")
  except Exception:
    return ""

meta = req_json(f"https://api.github.com/repos/{repo}")
try:
  contents = req_json(f"https://api.github.com/repos/{repo}/contents")
except Exception:
  contents = []

names = []
if isinstance(contents, list):
  for it in contents:
    if isinstance(it, dict) and "name" in it:
      names.append(str(it["name"]))

rd = readme_text(repo).lower()

score = 0
signals = []

if meta.get("archived") is True:
  score += 10
  signals.append("repo archived")
if meta.get("license") is None:
  score += 10
  signals.append("no license metadata")
if "Dockerfile" in names:
  score += 8
  signals.append("has Dockerfile")
if ".github" in names:
  score += 8
  signals.append("has .github directory")

if re.search(r"curl\s+[^|]*\|\s*(sh|bash)", rd):
  score += 45
  signals.append("README contains curl|sh pattern")
if re.search(r"bash\s+<\(\s*curl", rd):
  score += 45
  signals.append("README contains bash <(curl ...)")
if re.search(r"rm\s+-rf", rd):
  score += 50
  signals.append("README contains rm -rf")
if re.search(r"\bsudo\b", rd):
  score += 15
  signals.append("README contains sudo")

risk = "low"
if score >= 80:
  risk = "high"
elif score >= 40:
  risk = "medium"

out = {
  "ok": True,
  "riskLevel": risk,
  "score": int(score),
  "signals": signals,
}
sys.stdout.write(json.dumps(out, ensure_ascii=False))
PY

