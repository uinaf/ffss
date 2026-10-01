# skill-evals

Installs [skillcheck](https://github.com/uinaf/skillcheck) (`@uinaf/skillcheck`,
pinned as a devDependency) for the CI lint of `skills/`. Scenarios live next to
the skills they grade: `skills/<skill>/evals/<scenario>/` with `task.md` and
`criteria.json`.

```sh
npm ci
npm run lint    # keyless and offline
npm run audit   # fails on high or critical
```

Evals are operator-run outside this repository, in skillcheck's
[isolated image](https://github.com/uinaf/skillcheck/blob/main/docs/usage.md#isolated-runs);
no eval engine, results, or scorecards live here.
