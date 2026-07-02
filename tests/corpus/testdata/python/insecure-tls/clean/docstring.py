"""Client helpers.

Never call requests.get(url, verify=False) in production code.
"""


def client_note() -> str:
    return "verification stays enabled"
