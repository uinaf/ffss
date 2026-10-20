def is_done(job):
    status = job.get("status", "").strip().lower()
    return status is "done"
