import os
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parents[3]
env = os.environ.copy()
env["G1_RECONNECT"] = "1"
result = subprocess.run(
    ["go", "run", "./cmd/testdb", "go", "run", "./cmd/g1integration"],
    cwd=root / "Server/Backend", env=env, text=True,
    stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=180,
)
print(result.stdout)
if result.returncode or "G1_RECONNECT_INTEGRATION_PASS" not in result.stdout:
    raise SystemExit(f"G1_RECONNECT_INTEGRATION_FAIL exit={result.returncode}")
