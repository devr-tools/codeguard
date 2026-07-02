import os
import shlex
import subprocess

name = input('name? ')
os.system('echo ' + shlex.quote(name))

count = int(input('count? '))
os.system(f'head -n {count} log.txt')

subprocess.run(['echo', name])
