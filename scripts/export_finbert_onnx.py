#!/usr/bin/env python3
"""One-time: export ProsusAI/finbert to ONNX + copy tokenizer.json.
Produces dist/finbert.onnx and dist/tokenizer.json — upload both as a GitHub
release asset on the public `bourse` repo. Weights are never committed.

    pip install "optimum[exporters]" transformers torch
    python scripts/export_finbert_onnx.py
"""
import hashlib
import pathlib
import shutil
import subprocess

MODEL = "ProsusAI/finbert"
OUT = pathlib.Path("dist")
OUT.mkdir(exist_ok=True)

# optimum exports model + tokenizer into OUT
subprocess.run(
    ["optimum-cli", "export", "onnx", "--model", MODEL, "--task",
     "text-classification", str(OUT)],
    check=True,
)
# optimum names the graph model.onnx — rename to finbert.onnx
shutil.move(OUT / "model.onnx", OUT / "finbert.onnx")

for f in ("finbert.onnx", "tokenizer.json"):
    p = OUT / f
    digest = hashlib.sha256(p.read_bytes()).hexdigest()
    print(f"{digest}  {f}  ({p.stat().st_size} bytes)")
