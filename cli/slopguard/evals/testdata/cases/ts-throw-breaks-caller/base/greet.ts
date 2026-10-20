import { findUser } from "./users";

export function greeting(id: string): string {
  const user = findUser(id);
  if (user === undefined) {
    return "Hello, guest";
  }
  return `Hello, ${user.name}`;
}
