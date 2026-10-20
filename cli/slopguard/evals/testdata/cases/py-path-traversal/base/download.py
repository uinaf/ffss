import os

UPLOADS = "/srv/uploads"


def read_upload(name):
    if "/" in name:
        raise ValueError("invalid name")
    with open(os.path.join(UPLOADS, name), "rb") as handle:
        return handle.read()
