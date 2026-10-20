export function extension(name: string): string {
  const dot = name.lastIndexOf(".");
  if (dot < 0) return "";
  return name.substring(1);
}
