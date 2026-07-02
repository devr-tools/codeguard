# subprocess.run(cmd, shell=True) is dangerous; keep argument lists instead.
import subprocess

subprocess.run(["uptime"], check=True)
