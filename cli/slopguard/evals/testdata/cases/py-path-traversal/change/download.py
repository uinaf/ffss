import os

UPLOADS = "/srv/uploads"


def read_upload(name):
    """Read an upload; names may include sub-folders such as 2026/report.pdf."""
    with open(os.path.join(UPLOADS, name), "rb") as handle:
        return handle.read()
