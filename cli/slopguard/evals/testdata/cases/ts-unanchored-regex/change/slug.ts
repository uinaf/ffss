const slugPattern = /[a-z0-9]+(-[a-z0-9]+)*/;

export function isSlug(value: string): boolean {
  return slugPattern.test(value);
}
