"""Reuse the real G1 PostgreSQL/Backend/GS/two-client process runner."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import socket
import threading


def unauthorized_rpc(root, env):
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    logs = []
    ready = threading.Event()
    rejected = threading.Event()
    commands = []
    threads = []

    def read_log(process):
        for line in process.stdout:
            logs.append(line)
            if "G2_GUARD_LISTENING" in line:
                ready.set()
            if "G2_INPUT_REJECTED unauthorized" in line:
                rejected.set()

    try:
        for role in ["server", "client"]:
            process = subprocess.Popen(
                [env["G2_REAL_GODOT"], "--headless", "--path", str(root / "Server/GameServer"),
                 "--script", "res://tests/g2_unauthorized.gd"],
                env={**env, "G2_GUARD_ROLE": role, "G2_GUARD_PORT": str(port)},
                text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
            )
            commands.append(process)
            thread = threading.Thread(target=read_log, args=(process,))
            thread.start()
            threads.append(thread)
            if not ready.wait(10):
                raise RuntimeError("unauthorized probe did not listen")
        if not rejected.wait(10):
            raise RuntimeError("unauthorized native RPC was not rejected")
    finally:
        for process in commands:
            process.terminate()
            process.wait(timeout=5)
        for thread in threads:
            thread.join()
        print("".join(logs))
    if any("ERROR:" in line or "G2_FAIL" in line for line in logs):
        raise RuntimeError("unauthorized probe runtime error")

root = Path(__file__).resolve().parents[3]
env = os.environ.copy()
env["G2_REAL_GODOT"] = shutil.which("godot")
unauthorized_rpc(root, env)
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
