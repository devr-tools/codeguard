from Crypto.Cipher import AES


def encrypt(key: bytes, nonce: bytes, payload: bytes) -> bytes:
    cipher = AES.new(key, AES.MODE_GCM, nonce=nonce)
    return cipher.encrypt(payload)
