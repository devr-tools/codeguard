import subprocess
import sys


def read_target():
    return sys.argv[1]


def run_command(cmd):
    subprocess.run(cmd, shell=True)


def main():
    target = read_target()
    run_command(target)
