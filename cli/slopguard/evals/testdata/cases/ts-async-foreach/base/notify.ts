export interface Mailer {
  send(to: string, body: string): Promise<void>;
}

export async function notify(mailer: Mailer, recipients: string[], body: string): Promise<number> {
  return 0;
}
