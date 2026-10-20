def find_user(connection, name, active_only=False):
    query = f"SELECT id, name FROM users WHERE name = '{name}'"
    if active_only:
        query += " AND active = 1"
    cursor = connection.execute(query)
    return cursor.fetchone()
