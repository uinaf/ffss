quick look before I commit: is pageCount in src/paginate.ts right? don't change the file, just tell me

=============== FILE: src/paginate.ts ===============
export function pageCount(total: number, pageSize: number): number {
  if (pageSize <= 0) throw new RangeError("pageSize must be positive");
  return Math.floor(total / pageSize);
}

export function pageSlice<T>(items: T[], page: number, pageSize: number): T[] {
  const start = page * pageSize;
  return items.slice(start, start + pageSize);
}
=============== END FILE ===============
