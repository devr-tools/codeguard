import hashlib


def checksum(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()
