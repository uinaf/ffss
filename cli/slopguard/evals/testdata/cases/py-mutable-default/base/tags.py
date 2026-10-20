def add_tag(tag, tags=None):
    """Return tags with tag appended."""
    if tags is None:
        tags = []
    return tags + [tag]
