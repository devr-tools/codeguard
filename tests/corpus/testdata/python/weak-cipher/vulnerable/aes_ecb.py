from Crypto.Cipher import AES


def encrypt(key: bytes, payload: bytes) -> bytes:
    cipher = AES.new(key, AES.MODE_ECB)
    return cipher.encrypt(payload)
