# Invoicing Service — On-Call Runbook

## Common Issues

### Invoice generation failing

Check the service logs:
```bash
docker logs invoicing-service --tail 100
```

Verify the calculator is healthy by calling the health endpoint:
```bash
curl http://localhost:3000/api/v3/health
```

### Payment webhook failures

Webhook processing is handled by `src/api/webhooks.ts`. Common causes:

1. Signature mismatch — check `STRIPE_WEBHOOK_SECRET` in environment
2. Endpoint unreachable — verify `/api/v3/webhooks/stripe` is accessible
3. Database write failure — check `DATABASE_URL` and run `npx knex migrate:latest`
