# eval(user_input) would be dangerous here; JSON parsing is used instead.
import json


def parse(text: str):
    return json.loads(text)
