export interface User {
  id: string;
  name: string;
}

const users = new Map<string, User>();

export function findUser(id: string): User {
  const user = users.get(id);
  if (!user) {
    throw new Error(`unknown user ${id}`);
  }
  return user;
}
