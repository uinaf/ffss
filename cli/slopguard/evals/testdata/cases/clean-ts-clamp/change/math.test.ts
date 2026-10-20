import { clamp } from "./math";

test("clamp keeps values inside the range", () => {
  expect(clamp(5, 0, 10)).toBe(5);
  expect(clamp(-1, 0, 10)).toBe(0);
  expect(clamp(11, 0, 10)).toBe(10);
});

test("clamp rejects an inverted range", () => {
  expect(() => clamp(1, 10, 0)).toThrow(RangeError);
});
