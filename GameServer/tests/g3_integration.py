import os
from pathlib import Path
import subprocess
import tempfile

root = Path(__file__).resolve().parents[3]
with tempfile.TemporaryDirectory(prefix="g3-runner-") as temp:
    env = os.environ.copy()
    env["G3_INTEGRATION"] = "1"
    env["G3_BARRIER_DIR"] = temp
    result = subprocess.run(
        ["go", "run", "./cmd/testdb", "go", "run", "./cmd/g1integration"],
        cwd=root / "Server/Backend", env=env, text=True,
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=120,
    )
    print(result.stdout)
    if result.returncode or "G3_INTEGRATION_PASS" not in result.stdout:
        raise SystemExit(f"G3_INTEGRATION_FAIL exit={result.returncode}")
