def add_tag(tag, tags=[]):
    """Return tags with tag appended, skipping duplicates."""
    if tag not in tags:
        tags.append(tag)
    return tags
