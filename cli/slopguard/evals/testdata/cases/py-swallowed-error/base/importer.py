import csv


def import_rows(path, save):
    """Import every row from a CSV file, returning the number saved."""
    with open(path, newline="") as handle:
        rows = list(csv.DictReader(handle))
    for row in rows:
        save(row)
    return len(rows)
