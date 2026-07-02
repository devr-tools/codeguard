from ast import literal_eval


def parse_config(text: str):
    return literal_eval(text)
