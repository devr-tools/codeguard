import yaml


def load_config(handle):
    strict = yaml.load(handle, Loader=yaml.SafeLoader)
    relaxed = yaml.safe_load(handle)
    return strict or relaxed
