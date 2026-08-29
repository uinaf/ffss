# Replace Change-Detector Tests

## Problem/Feature Description

An agent added tests for our connection notice by copying its implementation
into assertions. Copy and CSS changes now break the suite even though the
product behavior has not changed. Clean `tests/connection-notice.test.ts` in
place without weakening the actual contract. Do not modify the module.

The stable contract is deliberately small: an offline connection notice is an
alert with kind `connection-error` and offers a `retry` action. Product copy,
CSS classes, object property order, and private source structure may change
without a behavior change.

## Input Files

=============== FILE: src/connection-notice.ts ===============
export interface ConnectionNotice {
  kind: "connection-error";
  role: "alert";
  className: string;
  title: string;
  body: string;
  action: {
    label: string;
    intent: "retry";
  };
}

export function connectionNotice(): ConnectionNotice {
  return {
    kind: "connection-error",
    role: "alert",
    className: "rounded border-red-400 bg-red-50",
    title: "Connection lost",
    body: "Check your internet connection and try again.",
    action: {
      label: "Try again",
      intent: "retry",
    },
  };
}
=============== END FILE ===============

=============== FILE: tests/connection-notice.test.ts ===============
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { connectionNotice } from "../src/connection-notice";

describe("connectionNotice", () => {
  it("contains the expected implementation", () => {
    const source = readFileSync(
      new URL("../src/connection-notice.ts", import.meta.url),
      "utf8",
    );

    expect(source).toContain('className: "rounded border-red-400 bg-red-50"');
    expect(source).toContain('title: "Connection lost"');
    expect(source).toContain('intent: "retry"');
  });

  it("renders the exact connection notice", () => {
    expect(connectionNotice()).toEqual({
      kind: "connection-error",
      role: "alert",
      className: "rounded border-red-400 bg-red-50",
      title: "Connection lost",
      body: "Check your internet connection and try again.",
      action: {
        label: "Try again",
        intent: "retry",
      },
    });
  });

  it("keeps the approved copy", () => {
    expect(connectionNotice().body).toBe(
      "Check your internet connection and try again.",
    );
    expect(connectionNotice().action.label).toBe("Try again");
  });

  it("matches the notice snapshot", () => {
    expect(connectionNotice()).toMatchInlineSnapshot(`
      {
        "action": {
          "intent": "retry",
          "label": "Try again",
        },
        "body": "Check your internet connection and try again.",
        "className": "rounded border-red-400 bg-red-50",
        "kind": "connection-error",
        "role": "alert",
        "title": "Connection lost",
      }
    `);
  });
});
=============== END FILE ===============
