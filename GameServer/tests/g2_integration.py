"""Reuse the real G1 PostgreSQL/Backend/GS/two-client process runner."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

root = Path(__file__).resolve().parents[3]
env = os.environ.copy()
env["G2_REAL_GODOT"] = shutil.which("godot")
with tempfile.TemporaryDirectory(prefix="g2-runner-") as temp:
    env["G2_BARRIER_DIR"] = temp
    wrapper = Path(__file__).with_name("g2_godot.sh")
    os.symlink(wrapper, Path(temp) / "godot")
    env["PATH"] = temp + os.pathsep + env["PATH"]
    result = subprocess.run(
        ["go", "run", "./cmd/testdb", "go", "run", "./cmd/g1integration"],
        cwd=root / "Server/Backend", env=env, text=True,
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=150,
    )
    print(result.stdout)
    required = ["G1_INTEGRATION_PASS", "G2_AUTHORITATIVE_CONFIRM", "G2_REMOTE_POSITION",
                "G2_INPUT_REJECTED future", "G2_INPUT_REJECTED direction"]
    missing = [marker for marker in required if marker not in result.stdout]
    if result.returncode or missing:
        raise SystemExit(f"G2_INTEGRATION_FAIL exit={result.returncode} missing={missing}")
    print("G2_INTEGRATION_PASS")
