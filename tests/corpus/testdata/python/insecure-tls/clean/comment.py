# Do not pass verify=False to requests calls; certificate checks stay on.
import requests


def fetch(url: str):
    return requests.get(url, timeout=10)
