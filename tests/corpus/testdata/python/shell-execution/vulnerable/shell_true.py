import subprocess


def run(cmd: str) -> int:
    return subprocess.run(cmd, shell=True).returncode
