# Detox an AI-Generated Test Suite

## Problem/Feature Description

An agent generated the test file below for our cart module. It pads coverage
numbers but tests almost nothing. Clean it up. Real coverage must not get
weaker: every behavior that has an assertion today must still be covered when
you are done, by a better test if not the same one. Edit
`tests/cart.test.ts` in place. The module under test is provided for
reference and must not be modified.

## Input Files

=============== FILE: src/cart.ts ===============
export interface Item {
  sku: string;
  price: number;
  qty: number;
}

export function total(items: Item[]): number {
  if (items.some((i) => i.price < 0 || i.qty < 0)) {
    throw new Error("negative price or qty");
  }
  return items.reduce((sum, i) => sum + i.price * i.qty, 0);
}

export function applyDiscount(totalCents: number, percent: number): number {
  if (percent < 0 || percent > 100) {
    throw new Error("percent out of range");
  }
  return Math.round(totalCents * (1 - percent / 100));
}
=============== END FILE ===============

=============== FILE: tests/cart.test.ts ===============
import { describe, it, expect, vi } from "vitest";
import { total, applyDiscount } from "../src/cart";

describe("cart", () => {
  it("total should be defined", () => {
    expect(total).toBeDefined();
  });

  it("total returns the total", () => {
    const items = [{ sku: "a", price: 100, qty: 2 }];
    expect(total(items)).toBe(total(items));
  });

  it("calculates total for one item", () => {
    const items = [{ sku: "a", price: 100, qty: 1 }];
    expect(total(items)).toBe(100);
  });

  it("calculates total for two items", () => {
    const items = [
      { sku: "a", price: 100, qty: 1 },
      { sku: "b", price: 200, qty: 1 },
    ];
    expect(total(items)).toBe(300);
  });

  it("calculates total for three items", () => {
    const items = [
      { sku: "a", price: 100, qty: 1 },
      { sku: "b", price: 200, qty: 1 },
      { sku: "c", price: 300, qty: 1 },
    ];
    expect(total(items)).toBe(600);
  });

  it("applies a discount using a mock", () => {
    const mockApplyDiscount = vi.fn().mockReturnValue(90);
    expect(mockApplyDiscount(100, 10)).toBe(90);
    expect(mockApplyDiscount).toHaveBeenCalledWith(100, 10);
  });

  it("throws on a negative quantity", () => {
    expect(() => total([{ sku: "a", price: 100, qty: -1 }])).toThrow(
      "negative price or qty",
    );
  });
});
=============== END FILE ===============
