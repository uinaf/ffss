# Enabling Parallel Agent Work on a Python Flask API

## Problem/Feature Description

Three agents work concurrently on the same Flask API repository in separate
git worktrees. They crash each other's instances because all start the server
on port 5000, and when something goes wrong they have no machine-readable
output to query, only console print statements. New worktrees also miss the
repo's gitignored `.env.local`, which agents need for local-only development
configuration.

Fix both problems so agents can run reliably in parallel. Keep lifecycle,
resource allocation, readiness, and state handling in a testable Python tool
rather than shell scripts.

## Output Specification

Produce the following files:

1. `app.py` — Flask application with an HTTP liveness check and
   machine-readable output for each request.

2. `tools/dev_service.py` — Python CLI with `start` and `stop` commands. `start`
   must allocate or acquire an isolated port, launch the Flask service, persist
   the owned process state, confirm bounded readiness, and exit non-zero after
   cleanup on failure. `stop` must clean up only the service owned by the current
   worktree.

3. `.gitignore` — Ignore local environment files and runtime artifacts.

4. `.worktreeinclude` — Narrowly include the gitignored local config file that
   managed Codex or Claude worktrees need.

5. `observability-notes.md` — Explain the health query, the resource-allocation
   owner, the persisted lifecycle state, managed-worktree config copying, and
   the teardown command.

Do not add shell wrappers for the Python CLI or reimplement JSON parsing,
readiness polling, PID ownership, or retry policy in shell.

## Input Files

Current state of the Flask API. Extract before beginning.

=============== FILE: app.py ===============
from flask import Flask, request, jsonify
import datetime

app = Flask(__name__)

tasks = []

@app.route('/tasks', methods=['GET'])
def get_tasks():
    print(f"GET /tasks called at {datetime.datetime.now()}")
    return jsonify(tasks)

@app.route('/tasks', methods=['POST'])
def create_task():
    data = request.get_json()
    task = {'id': len(tasks) + 1, **data}
    tasks.append(task)
    print(f"Created task: {task}")
    return jsonify(task), 201

@app.route('/tasks/<int:task_id>', methods=['DELETE'])
def delete_task(task_id):
    global tasks
    tasks = [t for t in tasks if t['id'] != task_id]
    print(f"Deleted task {task_id}")
    return '', 204

if __name__ == '__main__':
    app.run(port=5000)

=============== FILE: requirements.txt ===============
flask>=3.0.0

=============== FILE: README.md ===============
# Task Manager API

Start with: python app.py
