import pickle


def load_session(blob: bytes):
    return pickle.loads(blob)
