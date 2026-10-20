from limits import validate_page_size

DEFAULT_PAGE_SIZE = 250


def list_page(fetch, size=DEFAULT_PAGE_SIZE):
    return fetch(validate_page_size(size))
