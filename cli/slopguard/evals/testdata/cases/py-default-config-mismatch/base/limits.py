MAX_PAGE_SIZE = 100


def validate_page_size(size):
    if not 1 <= size <= MAX_PAGE_SIZE:
        raise ValueError(f"page size must be between 1 and {MAX_PAGE_SIZE}")
    return size
