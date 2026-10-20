export interface User {
  id: string;
  name: string;
}

const users = new Map<string, User>();

export function findUser(id: string): User | undefined {
  return users.get(id);
}
