import os

import requests

url = os.environ["TARGET"]
response = requests.get(url, timeout=5)
