import requests


def fetch_report(url: str):
    return requests.get(url, verify=False, timeout=10)
