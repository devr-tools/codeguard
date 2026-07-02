def lookup(cursor, user_id):
    return cursor.execute("SELECT * FROM users WHERE id = %s", (user_id,))
