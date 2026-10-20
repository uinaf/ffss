def find_user(connection, name):
    cursor = connection.execute("SELECT id, name FROM users WHERE name = ?", (name,))
    return cursor.fetchone()
