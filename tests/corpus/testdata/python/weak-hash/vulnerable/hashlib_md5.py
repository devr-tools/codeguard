import hashlib


def checksum(data: bytes) -> str:
    return hashlib.md5(data).hexdigest()
