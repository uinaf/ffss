from events import build_event


def audit_line(user_id, action):
    event = build_event(user_id, action)
    return f"{event['user_id']} did {event['action']}"
