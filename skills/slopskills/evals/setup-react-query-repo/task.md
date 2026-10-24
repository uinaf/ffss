setting this repo up so codex and claude code work well in it. which stack skills should it have? don't install anything yet, just give me the exact commands and why each one fits

=============== FILE: package.json ===============
{
  "name": "orders-dashboard",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "test": "vitest run"
  },
  "dependencies": {
    "@tanstack/react-query": "^5.62.0",
    "@tanstack/react-router": "^1.95.0",
    "react": "^19.0.0",
    "react-dom": "^19.0.0"
  },
  "devDependencies": {
    "@vitejs/plugin-react": "^4.3.4",
    "typescript": "^5.7.2",
    "vite": "^6.0.7",
    "vitest": "^2.1.8"
  }
}
=============== END FILE ===============

=============== FILE: src/main.tsx ===============
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createRouter } from "@tanstack/react-router";
import { createRoot } from "react-dom/client";
import { routeTree } from "./routeTree.gen";

const queryClient = new QueryClient();
const router = createRouter({ routeTree, context: { queryClient } });

createRoot(document.getElementById("root")!).render(
  <QueryClientProvider client={queryClient}>
    <RouterProvider router={router} />
  </QueryClientProvider>,
);
=============== END FILE ===============

=============== FILE: AGENTS.md ===============
# Agent Guide

Internal dashboard for order operations. `pnpm dev` runs it; `pnpm test` is
the gate.
=============== END FILE ===============

=============== FILE: CLAUDE.md ===============
@AGENTS.md
=============== END FILE ===============
