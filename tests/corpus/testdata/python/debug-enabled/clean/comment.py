# Set debug = True only on a local workstation, never in deployed configs.
from flask import Flask

app = Flask(__name__)
